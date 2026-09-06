package entproto

import (
	"fmt"
	"hash/fnv"

	"entgo.io/ent/entc/gen"
	"github.com/go-viper/mapstructure/v2"
)

// FixGraph automatically adds entproto annotations to schemas that don't have them.
// It adds Message annotation to schemas, Field annotation to fields and edges.
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
	for j := range node.Fields {
		if err := addAnnotationForField(node.Fields[j], idGenerator, "field"); err != nil {
			return err
		}
	}

	// Add annotation for edges
	for j := range node.Edges {
		if err := addAnnotationForEdge(node.Edges[j], idGenerator); err != nil {
			return err
		}
	}
	return nil
}

func addAnnotationForEdge(ed *gen.Edge, idGenerator *fieldIDGenerator) error {
	if ed.Annotations == nil {
		ed.Annotations = make(map[string]any, 1)
	}
	if ed.Annotations[FieldAnnotation] != nil {
		return nil
	}
	if ed.Annotations[SkipAnnotation] != nil {
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
	if fd.Annotations[FieldAnnotation] != nil {
		return nil
	}
	if fd.Annotations[SkipAnnotation] != nil {
		return nil
	}

	if _, occupied := idGenerator.exist[IDFieldNumber]; occupied {
		return fmt.Errorf("entproto: field number %d is already occupied in schema %q; annotate the ID and conflicting field explicitly", IDFieldNumber, idGenerator.schema)
	}
	fd.Annotations[FieldAnnotation] = Field(IDFieldNumber)
	idGenerator.exist[IDFieldNumber] = struct{}{}
	return nil
}

func addAnnotationForField(fd *gen.Field, idGenerator *fieldIDGenerator, kind string) error {
	if fd.Annotations == nil {
		fd.Annotations = make(map[string]any, 1)
	}
	if fd.Annotations[FieldAnnotation] != nil || fd.Annotations[SkipAnnotation] != nil {
		return nil
	}

	num, err := idGenerator.Next(kind, fd.Name)
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
	num := int(h.Sum32()%uint32(536870911-1)) + 2
	if num >= 19000 && num <= 19999 {
		num += 1000
	}
	return num
}

func extractExistFieldID(node *gen.Type) (map[int]struct{}, error) {
	existNums := map[int]struct{}{}
	fields := make([]*gen.Field, 0, len(node.Fields)+1)
	fields = append(fields, node.ID)
	fields = append(fields, node.Fields...)
	for _, fd := range fields {
		if fd.Annotations != nil {
			if obj, exist := fd.Annotations[FieldAnnotation]; exist {
				pbField := struct {
					Number int
				}{}
				err := mapstructure.Decode(obj, &pbField)
				if err != nil {
					return nil, &InvalidAnnotationError{
						Schema:     node.Name,
						Field:      fd.Name,
						Annotation: FieldAnnotation,
						Cause:      err,
					}
				}
				existNums[pbField.Number] = struct{}{}
			}
		}
	}
	// Also check edges
	for _, ed := range node.Edges {
		if ed.Annotations != nil {
			if obj, exist := ed.Annotations[FieldAnnotation]; exist {
				pbField := struct {
					Number int
				}{}
				err := mapstructure.Decode(obj, &pbField)
				if err != nil {
					return nil, &InvalidAnnotationError{
						Schema:     node.Name,
						Edge:       ed.Name,
						Annotation: FieldAnnotation,
						Cause:      err,
					}
				}
				existNums[pbField.Number] = struct{}{}
			}
		}
	}
	return existNums, nil
}
