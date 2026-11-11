package entproto

import (
	"fmt"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// customTypeEntry describes an externally-defined protobuf message or enum
// referenced by an ent schema. The proto file at ProtoFile is owned by the
// user and is never written by entproto.
type customTypeEntry struct {
	ProtoFile    string
	ProtoPackage string
	Path         string // name relative to ProtoPackage, such as "Outer.Inner"
	Enum         bool
}

func (e customTypeEntry) kind() string {
	if e.Enum {
		return "an enum"
	}
	return "a message"
}

var (
	// customTypeRegistry holds the types registered with RegisterCustomType
	// and RegisterCustomEnum, keyed by full name without a leading dot. It is
	// process-wide so the entc program can register types once for every
	// Extension; types referenced by MessageField and EnumField annotations
	// are resolved per Adapter instead and never stored here.
	customTypeRegistryMu sync.RWMutex
	customTypeRegistry   = map[string]customTypeEntry{}
)

// RegisterCustomType registers an externally-defined protobuf message type by
// reflecting on the supplied generated Go message. The fully-qualified type
// name and proto file path are read straight from the descriptor, so they
// cannot drift from the actual .pb.go contract.
//
// Once registered, a [Field] annotation can reference the type by name:
//
//	entproto.Field(3,
//		entproto.Type(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
//		entproto.TypeName("shared.v1.User"),
//	)
//
// entproto will emit the appropriate `import "shared/v1/user.proto";` in the
// generated .proto file and reference the field as `shared.v1.User`.
// [MessageField] does the same in one step without a registration.
//
// The registry is process-wide and safe for concurrent use; registrations
// persist for the life of the process and are shared by every [Extension].
// Register types in the entc program before running code generation, from its
// main or an init function; ent schema packages run in a separate process
// while the graph is loaded, so registering there has no effect.
//
// RegisterCustomType panics if msg is nil, has no file descriptor, or is
// declared in a .proto file without a package statement, or if the same type
// name is already registered with a different file or package; calling it
// again with the same type is a no-op.
func RegisterCustomType(msg proto.Message) {
	fullName, entry := describeMessage("RegisterCustomType", msg)
	registerCustomType(fullName, entry)
}

// RegisterCustomEnum is the enum counterpart of [RegisterCustomType]: it
// registers an externally-defined protobuf enum, given any value of its
// generated Go type, so a [Field] annotation with [Type] TYPE_ENUM and
// [TypeName] can reference it. [EnumField] does the same in one step.
//
// It panics under the same conditions as RegisterCustomType, and also when
// the enum is closed (declared in a proto2 file), because proto3 messages
// cannot use closed enums.
func RegisterCustomEnum(enum protoreflect.Enum) {
	fullName, entry := describeEnum("RegisterCustomEnum", enum)
	registerCustomType(fullName, entry)
}

// describeMessage locates msg's type for [RegisterCustomType] and
// [MessageField]. caller names the exported function in panic messages.
func describeMessage(caller string, msg proto.Message) (string, customTypeEntry) {
	if msg == nil {
		panic("entproto: " + caller + " called with nil message")
	}
	return describeType(caller, msg, msg.ProtoReflect().Descriptor(), false)
}

// describeEnum locates enum's type for [RegisterCustomEnum] and [EnumField].
func describeEnum(caller string, enum protoreflect.Enum) (string, customTypeEntry) {
	if enum == nil {
		panic("entproto: " + caller + " called with nil enum")
	}
	d := enum.Descriptor()
	if d != nil && d.IsClosed() {
		panic(fmt.Sprintf("entproto: %s(%T): %s is a closed (proto2) enum, which proto3 messages cannot use",
			caller, enum, d.FullName()))
	}
	return describeType(caller, enum, d, true)
}

func describeType(caller string, goValue any, d protoreflect.Descriptor, isEnum bool) (string, customTypeEntry) {
	if d == nil {
		panic(fmt.Sprintf("entproto: %s(%T) returned nil descriptor", caller, goValue))
	}
	parent := d.ParentFile()
	if parent == nil {
		panic(fmt.Sprintf("entproto: %s(%T) descriptor has no parent file", caller, goValue))
	}
	pkg := string(parent.Package())
	if pkg == "" {
		panic(fmt.Sprintf("entproto: %s(%T): %s is declared in %q, which has no package statement;"+
			" types from .proto files without a package are not supported", caller, goValue, d.FullName(), parent.Path()))
	}
	fullName := string(d.FullName())
	return fullName, customTypeEntry{
		ProtoFile:    parent.Path(),
		ProtoPackage: pkg,
		Path:         strings.TrimPrefix(fullName, pkg+"."),
		Enum:         isEnum,
	}
}

func registerCustomType(fullName string, entry customTypeEntry) {
	customTypeRegistryMu.Lock()
	defer customTypeRegistryMu.Unlock()
	if existing, ok := customTypeRegistry[fullName]; ok {
		if existing == entry {
			return
		}
		panic(customTypeConflict(fullName, existing, entry).Error())
	}
	customTypeRegistry[fullName] = entry
}

func customTypeConflict(fullName string, existing, entry customTypeEntry) error {
	return fmt.Errorf("entproto: custom type %q is already registered as %s in %q, cannot re-register it as %s in %q",
		fullName, existing.kind(), existing.ProtoFile, entry.kind(), entry.ProtoFile)
}

// lookupRegisteredCustomType returns the process-wide registry entry for the
// given fully-qualified proto type name. The name may carry a leading dot.
func lookupRegisteredCustomType(protoTypeName string) (customTypeEntry, bool) {
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

// addCustomTypeToStub declares the external type in the stub
// FileDescriptorProto for its proto file, creating the stub if needed. Stubs
// are fed into desc.CreateFileDescriptors so cross-file references can link,
// but are filtered out before printing so the user's real file is never
// overwritten.
func addCustomTypeToStub(files map[string]*descriptorpb.FileDescriptorProto, entry customTypeEntry) error {
	stub, ok := files[entry.ProtoFile]
	if !ok {
		stub = &descriptorpb.FileDescriptorProto{
			Name:    toPtr(entry.ProtoFile),
			Package: toPtr(entry.ProtoPackage),
			Syntax:  toPtr("proto3"),
		}
		files[entry.ProtoFile] = stub
	} else if stub.GetPackage() != entry.ProtoPackage {
		return fmt.Errorf("entproto: external proto file %q registered with conflicting packages %q and %q", entry.ProtoFile, stub.GetPackage(), entry.ProtoPackage)
	}
	path := strings.Split(entry.Path, ".")
	if !entry.Enum {
		stub.MessageType, _ = addMessagePath(stub.MessageType, path)
		return nil
	}
	parents, name := path[:len(path)-1], path[len(path)-1]
	if len(parents) == 0 {
		stub.EnumType = addStubEnum(stub.EnumType, name, entry.Path)
		return nil
	}
	var parent *descriptorpb.DescriptorProto
	stub.MessageType, parent = addMessagePath(stub.MessageType, parents)
	parent.EnumType = addStubEnum(parent.EnumType, name, entry.Path)
	return nil
}

// addMessagePath declares the (possibly nested) message at path and returns
// the updated list together with the innermost message.
func addMessagePath(messages []*descriptorpb.DescriptorProto, path []string) ([]*descriptorpb.DescriptorProto, *descriptorpb.DescriptorProto) {
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
	if len(path) == 1 {
		return messages, message
	}
	var leaf *descriptorpb.DescriptorProto
	message.NestedType, leaf = addMessagePath(message.NestedType, path[1:])
	return messages, leaf
}

// addStubEnum declares the enum name unless it already exists. proto3 enums
// need a zero value; the stub's only has to be unique within its scope.
func addStubEnum(enums []*descriptorpb.EnumDescriptorProto, name, path string) []*descriptorpb.EnumDescriptorProto {
	for _, enum := range enums {
		if enum.GetName() == name {
			return enums
		}
	}
	zero := "ENTPROTO_STUB_" + strings.ToUpper(strings.ReplaceAll(path, ".", "_")) + "_ZERO"
	return append(enums, &descriptorpb.EnumDescriptorProto{
		Name: toPtr(name),
		Value: []*descriptorpb.EnumValueDescriptorProto{
			{Name: toPtr(zero), Number: toPtr[int32](0)},
		},
	})
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
