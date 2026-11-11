package entproto

import (
	"fmt"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// customTypeEntry describes an externally-defined protobuf message type that
// is referenced by an ent schema via entproto.Field(..., entproto.Type(TYPE_MESSAGE),
// entproto.TypeName(...)). The proto file at ProtoFile is owned by the user and
// will not be (over)written by entproto's generator.
type customTypeEntry struct {
	ProtoFile    string
	ProtoPackage string
	MessageName  string
	MessagePath  string
}

var (
	// customTypeRegistry is a process-wide registry of externally-defined
	// protobuf message types. It is intentionally global because annotations
	// are declared in ent schemas (package-level data) and must be resolvable
	// when LoadAdapter runs, without threading state through every schema.
	//
	// This has two consequences callers should be aware of:
	//   - registering the same type name with a different proto file path in
	//     the same process panics (see registerCustomTypeDetails); and
	//   - the registry is shared across Extension instances and test runs
	//     (resetCustomTypeRegistry exists for tests).
	customTypeRegistryMu sync.RWMutex
	customTypeRegistry   = map[string]customTypeEntry{}
)

// RegisterCustomType registers an externally-defined protobuf message type by
// reflecting on the supplied generated Go message. The fully-qualified type
// name and proto file path are read straight from the descriptor, so they
// cannot drift from the actual .pb.go contract.
//
// Once registered, an ent schema can reference that type from a Field
// annotation (most conveniently via entproto.MessageField):
//
//	field.JSON("user", &sharedv1.User{}).
//		Annotations(entproto.MessageField(3, &sharedv1.User{}))
//
// entproto will emit the appropriate `import "shared/v1/user.proto";` in the
// generated .proto file and reference the field as `shared.v1.User`.
//
// The registry is process-wide and safe for concurrent use; registrations
// persist for the life of the process and are shared by every [Extension].
// Register types before running code generation, typically from the entc
// program's main or an init function.
//
// RegisterCustomType panics if msg is nil or has no file descriptor, or if the
// same type name is already registered with a different file or package;
// calling it again with the same type is a no-op.
func RegisterCustomType(msg proto.Message) {
	entry := describeMessage("RegisterCustomType", msg)
	registerCustomTypeDetails(entry.FullName, entry.ProtoFile, entry.ProtoPackage, entry.MessagePath)
}

// messageInfo locates a generated protobuf message within its .proto file.
type messageInfo struct {
	FullName     string
	ProtoFile    string
	ProtoPackage string
	MessagePath  string // message name relative to ProtoPackage, e.g. "Outer.Inner"
}

// describeMessage reads msg's descriptor. caller names the exported function
// in panic messages.
func describeMessage(caller string, msg proto.Message) messageInfo {
	if msg == nil {
		panic("entproto: " + caller + " called with nil message")
	}
	d := msg.ProtoReflect().Descriptor()
	if d == nil {
		panic(fmt.Sprintf("entproto: %s(%T) returned nil descriptor", caller, msg))
	}
	parent := d.ParentFile()
	if parent == nil {
		panic(fmt.Sprintf("entproto: %s(%T) descriptor has no parent file", caller, msg))
	}
	fullName := string(d.FullName())
	return messageInfo{
		FullName:     fullName,
		ProtoFile:    parent.Path(),
		ProtoPackage: string(parent.Package()),
		MessagePath:  strings.TrimPrefix(fullName, string(parent.Package())+"."),
	}
}

func registerCustomTypeDetails(protoTypeName, protoFilePath, protoPackage, messagePath string) {
	name := strings.TrimPrefix(protoTypeName, ".")
	messagePath = strings.TrimPrefix(messagePath, ".")
	if name == "" || protoFilePath == "" || protoPackage == "" || messagePath == "" {
		panic(fmt.Sprintf("entproto: incomplete custom type registration for %q", protoTypeName))
	}
	parts := strings.Split(messagePath, ".")
	entry := customTypeEntry{
		ProtoFile:    protoFilePath,
		ProtoPackage: protoPackage,
		MessageName:  parts[len(parts)-1],
		MessagePath:  messagePath,
	}
	customTypeRegistryMu.Lock()
	defer customTypeRegistryMu.Unlock()
	if existing, ok := customTypeRegistry[name]; ok {
		if existing == entry {
			return
		}
		panic(fmt.Sprintf("entproto: custom type %q already registered with proto file %q, cannot re-register with %q",
			protoTypeName, existing.ProtoFile, protoFilePath))
	}
	customTypeRegistry[name] = entry
}

// lookupCustomType returns the registry entry for the given fully-qualified
// proto type name. The name may carry an optional leading dot.
func lookupCustomType(protoTypeName string) (customTypeEntry, bool) {
	name := strings.TrimPrefix(protoTypeName, ".")
	customTypeRegistryMu.RLock()
	defer customTypeRegistryMu.RUnlock()
	entry, ok := customTypeRegistry[name]
	return entry, ok
}

// resetCustomTypeRegistry clears the registry; intended for tests only.
func resetCustomTypeRegistry() {
	customTypeRegistryMu.Lock()
	defer customTypeRegistryMu.Unlock()
	customTypeRegistry = map[string]customTypeEntry{}
}

// addCustomTypeToStub declares the registered message in the stub
// FileDescriptorProto for its proto file, creating the stub if needed. Stubs
// are fed into desc.CreateFileDescriptors so cross-file references can link,
// but are filtered out before printing so the user's real file is never
// overwritten.
func addCustomTypeToStub(files map[string]*descriptorpb.FileDescriptorProto, entry customTypeEntry) error {
	stub, ok := files[entry.ProtoFile]
	if !ok {
		file := entry.ProtoFile
		pkg := entry.ProtoPackage
		stub = &descriptorpb.FileDescriptorProto{
			Name:    &file,
			Package: &pkg,
			Syntax:  toPtr("proto3"),
		}
		files[entry.ProtoFile] = stub
	} else if stub.GetPackage() != entry.ProtoPackage {
		return fmt.Errorf("entproto: external proto file %q registered with conflicting packages %q and %q", entry.ProtoFile, stub.GetPackage(), entry.ProtoPackage)
	}
	stub.MessageType = addMessagePath(stub.MessageType, strings.Split(entry.MessagePath, "."))
	return nil
}

func addMessagePath(messages []*descriptorpb.DescriptorProto, path []string) []*descriptorpb.DescriptorProto {
	name := path[0]
	var message *descriptorpb.DescriptorProto
	for _, candidate := range messages {
		if candidate.GetName() == name {
			message = candidate
			break
		}
	}
	if message == nil {
		message = &descriptorpb.DescriptorProto{Name: toPtr(name)}
		messages = append(messages, message)
	}
	if len(path) > 1 {
		message.NestedType = addMessagePath(message.NestedType, path[1:])
	}
	return messages
}

// normalizeCustomTypeName ensures a proto type name is in the FQN form expected
// by protoreflect: a leading dot followed by the fully-qualified path. It is
// safe to call on user-supplied strings that may or may not already include
// the leading dot.
func normalizeCustomTypeName(name string) string {
	if name == "" {
		return name
	}
	if strings.HasPrefix(name, ".") {
		return name
	}
	return "." + name
}
