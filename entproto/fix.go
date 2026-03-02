package entproto

import (
	"fmt"
	"sort"

	"entgo.io/ent/entc/gen"
	"github.com/go-viper/mapstructure/v2"
)

// FixGraph automatically adds entproto annotations to schemas that don't have them.
// It adds Message annotation to schemas, Field annotation to fields and edges.
func FixGraph(g *gen.Graph) {
	for _, node := range g.Nodes {
		fixNode(node)
	}
}

func fixNode(node *gen.Type) {
	if node.Annotations == nil {
		node.Annotations = make(map[string]any, 1)
	}
	if node.Annotations[MessageAnnotation] != nil {
		return
	}
	// If the node does not have the message annotation, add it.
	node.Annotations[MessageAnnotation] = Message()

	idGenerator := &fieldIDGenerator{exist: extractExistFieldID(node)}

	// Sort fields: own fields first, then mixed-in fields
	sort.Slice(node.Fields, func(i, j int) bool {
		if node.Fields[i].Position.MixedIn != node.Fields[j].Position.MixedIn {
			return !node.Fields[i].Position.MixedIn
		}
		return node.Fields[i].Position.Index < node.Fields[j].Position.Index
	})

	// Add annotation for ID field
	addAnnotationForField(node.ID, idGenerator)

	// Add annotation for other fields
	for j := range node.Fields {
		addAnnotationForField(node.Fields[j], idGenerator)
	}

	// Add annotation for edges
	for j := range node.Edges {
		addAnnotationForEdge(node.Edges[j], idGenerator)
	}
}

func addAnnotationForEdge(ed *gen.Edge, idGenerator *fieldIDGenerator) {
	if ed.Annotations == nil {
		ed.Annotations = make(map[string]any, 1)
	}
	if ed.Annotations[FieldAnnotation] != nil {
		return
	}
	if ed.Annotations[SkipAnnotation] != nil {
		return
	}
	ed.Annotations[FieldAnnotation] = Field(idGenerator.MustNext())
}

func addAnnotationForField(fd *gen.Field, idGenerator *fieldIDGenerator) {
	if fd.Annotations == nil {
		fd.Annotations = make(map[string]any, 1)
	}
	if fd.Annotations[FieldAnnotation] != nil {
		return
	}
	if fd.Annotations[SkipAnnotation] != nil {
		return
	}

	fd.Annotations[FieldAnnotation] = Field(idGenerator.MustNext())
}

type fieldIDGenerator struct {
	current int
	exist   map[int]struct{}
}

func (f *fieldIDGenerator) Next() (int, error) {
	f.current++
	for {
		if _, ok := f.exist[f.current]; ok {
			f.current++
			continue
		}
		if f.current > 536870911 {
			return 0, fmt.Errorf("entproto: field number exceeds the maximum value 536870911")
		}
		break
	}
	return f.current, nil
}

func (f *fieldIDGenerator) MustNext() int {
	num, err := f.Next()
	if err != nil {
		panic(err)
	}
	return num
}

func extractExistFieldID(node *gen.Type) map[int]struct{} {
	existNums := map[int]struct{}{}
	for _, fd := range node.Fields {
		if fd.Annotations != nil {
			if obj, exist := fd.Annotations[FieldAnnotation]; exist {
				pbField := struct {
					Number int
				}{}
				err := mapstructure.Decode(obj, &pbField)
				if err != nil {
					panic(fmt.Errorf("entproto: failed decoding field annotation: %w", err))
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
					panic(fmt.Errorf("entproto: failed decoding edge annotation: %w", err))
				}
				existNums[pbField.Number] = struct{}{}
			}
		}
	}
	return existNums
}
