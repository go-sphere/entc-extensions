package entproto

import (
	"sort"

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
		return nil
	}
	// If the node does not have the message annotation, add it.
	node.Annotations[MessageAnnotation] = Message()

	exist, err := extractExistFieldID(node)
	if err != nil {
		return err
	}
	idGenerator := &fieldIDGenerator{schema: node.Name, exist: exist}

	// Sort fields: own fields first, then mixed-in fields
	sort.Slice(node.Fields, func(i, j int) bool {
		if node.Fields[i].Position.MixedIn != node.Fields[j].Position.MixedIn {
			return !node.Fields[i].Position.MixedIn
		}
		return node.Fields[i].Position.Index < node.Fields[j].Position.Index
	})

	// Add annotation for ID field
	if err := addAnnotationForField(node.ID, idGenerator); err != nil {
		return err
	}

	// Add annotation for other fields
	for j := range node.Fields {
		if err := addAnnotationForField(node.Fields[j], idGenerator); err != nil {
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
	num, err := idGenerator.Next(ed.Name)
	if err != nil {
		return err
	}
	ed.Annotations[FieldAnnotation] = Field(num)
	return nil
}

func addAnnotationForField(fd *gen.Field, idGenerator *fieldIDGenerator) error {
	if fd.Annotations == nil {
		fd.Annotations = make(map[string]any, 1)
	}
	if fd.Annotations[FieldAnnotation] != nil {
		return nil
	}
	if fd.Annotations[SkipAnnotation] != nil {
		return nil
	}

	num, err := idGenerator.Next(fd.Name)
	if err != nil {
		return err
	}

	fd.Annotations[FieldAnnotation] = Field(num)
	return nil
}

type fieldIDGenerator struct {
	schema  string
	current int
	exist   map[int]struct{}
}

func (f *fieldIDGenerator) Next(field string) (int, error) {
	f.current++
	for {
		if _, ok := f.exist[f.current]; ok {
			f.current++
			continue
		}
		if f.current > 536870911 {
			return 0, &FieldNumberOverflowError{
				Schema: f.schema,
				Field:  field,
				Number: f.current,
			}
		}
		break
	}
	return f.current, nil
}

func extractExistFieldID(node *gen.Type) (map[int]struct{}, error) {
	existNums := map[int]struct{}{}
	for _, fd := range node.Fields {
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
