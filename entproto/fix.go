package entproto

import (
	"fmt"
	"hash/fnv"

	"entgo.io/ent/entc/gen"
	"github.com/go-viper/mapstructure/v2"
)

// FixGraph adds missing entproto annotations to g in place; it is what
// [WithAutoFill] runs before generation. For every schema without a [Message]
// annotation it adds Message(). For every schema whose Message annotation has
// Generate=true (that is, not [SkipGen]) it then numbers each member that has
// neither a [Field] nor a [Skip] annotation: the ID gets [IDFieldNumber], and
// other fields and edges get a number in the valid protobuf range derived from
// a stable hash of the schema name, member kind, and member name, so numbers do
// not change when members are added, removed, or reordered.
//
// FixGraph stops at the first schema it cannot fix and returns an error: an
// [*InvalidAnnotationError] for an undecodable Field annotation, or an error
// when a derived number (or 1 for the ID) is already in use. Schemas processed
// before the failure stay modified.
func FixGraph(g *gen.Graph) error {
	for _, node := range g.Nodes {
		if err := fixNode(node); err != nil {
			return err
		}
	}
	return nil
}

func fixNode(node *gen.Type) error {
	if node.Annotations == nil {
		node.Annotations = make(map[string]any, 1)
	}
	if node.Annotations[MessageAnnotation] != nil {
		msg, err := extractMessageAnnotation(node)
		if err != nil {
			return err
		}
		if !msg.Generate {
			return nil
		}
	} else {
		// If the node does not have the message annotation, add it.
		node.Annotations[MessageAnnotation] = Message()
	}

	exist, err := extractExistFieldID(node)
	if err != nil {
		return err
	}
	idGenerator := &fieldIDGenerator{schema: node.Name, exist: exist}

	// Add annotation for ID field
	if err := addAnnotationForID(node.ID, idGenerator); err != nil {
		return err
	}

	// Add annotation for other fields
	for _, fd := range node.Fields {
		if err := addAnnotationForField(fd, idGenerator); err != nil {
			return err
		}
	}

	// Add annotation for edges
	for _, ed := range node.Edges {
		if err := addAnnotationForEdge(ed, idGenerator); err != nil {
			return err
		}
	}
	return nil
}

func addAnnotationForEdge(ed *gen.Edge, idGenerator *fieldIDGenerator) error {
	if ed.Annotations == nil {
		ed.Annotations = make(map[string]any, 1)
	}
	if ed.Annotations[FieldAnnotation] != nil || ed.Annotations[SkipAnnotation] != nil {
		return nil
	}
	num, err := idGenerator.Next("edge", ed.Name)
	if err != nil {
		return err
	}
	ed.Annotations[FieldAnnotation] = Field(num)
	return nil
}

func addAnnotationForID(fd *gen.Field, idGenerator *fieldIDGenerator) error {
	if fd.Annotations == nil {
		fd.Annotations = make(map[string]any, 1)
	}
	if fd.Annotations[FieldAnnotation] != nil || fd.Annotations[SkipAnnotation] != nil {
		return nil
	}

	if _, occupied := idGenerator.exist[IDFieldNumber]; occupied {
		return fmt.Errorf("entproto: field number %d is already occupied in schema %q; annotate the ID and conflicting field explicitly", IDFieldNumber, idGenerator.schema)
	}
	fd.Annotations[FieldAnnotation] = Field(IDFieldNumber)
	idGenerator.exist[IDFieldNumber] = struct{}{}
	return nil
}

func addAnnotationForField(fd *gen.Field, idGenerator *fieldIDGenerator) error {
	if fd.Annotations == nil {
		fd.Annotations = make(map[string]any, 1)
	}
	if fd.Annotations[FieldAnnotation] != nil || fd.Annotations[SkipAnnotation] != nil {
		return nil
	}

	num, err := idGenerator.Next("field", fd.Name)
	if err != nil {
		return err
	}
	fd.Annotations[FieldAnnotation] = Field(num)
	return nil
}

type fieldIDGenerator struct {
	schema string
	exist  map[int]struct{}
}

func (f *fieldIDGenerator) Next(kind, field string) (int, error) {
	num := stableFieldNumber(f.schema, kind, field)
	if _, occupied := f.exist[num]; occupied {
		return 0, fmt.Errorf(
			"entproto: stable field number %d for %s %q in schema %q is already occupied; annotate the colliding members explicitly",
			num, kind, field, f.schema,
		)
	}
	f.exist[num] = struct{}{}
	return num, nil
}

func stableFieldNumber(schema, kind, field string) int {
	h := fnv.New32a()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s", schema, kind, field)
	// Map into protobuf's valid user range while keeping 1 reserved for the ID.
	num := int(h.Sum32()%uint32(maxFieldNumber-1)) + 2
	if num >= reservedFieldNumberStart && num <= reservedFieldNumberEnd {
		num += reservedFieldNumberEnd - reservedFieldNumberStart + 1
	}
	return num
}

func extractExistFieldID(node *gen.Type) (map[int]struct{}, error) {
	existNums := map[int]struct{}{}
	for _, fd := range append([]*gen.Field{node.ID}, node.Fields...) {
		num, ok, err := annotatedFieldNumber(fd.Annotations)
		if err != nil {
			return nil, &InvalidAnnotationError{Schema: node.Name, Field: fd.Name, Annotation: FieldAnnotation, Cause: err}
		}
		if ok {
			existNums[num] = struct{}{}
		}
	}
	for _, ed := range node.Edges {
		num, ok, err := annotatedFieldNumber(ed.Annotations)
		if err != nil {
			return nil, &InvalidAnnotationError{Schema: node.Name, Edge: ed.Name, Annotation: FieldAnnotation, Cause: err}
		}
		if ok {
			existNums[num] = struct{}{}
		}
	}
	return existNums, nil
}

// annotatedFieldNumber returns the number of the Field annotation in
// annotations; ok is false when there is none.
func annotatedFieldNumber(annotations map[string]any) (num int, ok bool, err error) {
	obj, exist := annotations[FieldAnnotation]
	if !exist {
		return 0, false, nil
	}
	var pbField struct{ Number int }
	if err := mapstructure.Decode(obj, &pbField); err != nil {
		return 0, false, err
	}
	return pbField.Number, true, nil
}
