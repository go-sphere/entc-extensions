package entproto

import (
	"entgo.io/ent/schema"
)

// SkipAnnotation is the annotation name under which [Skip] is stored in ent
// field and edge annotations.
const SkipAnnotation = "ProtoSkip"

type skipped struct{}

// Skip annotates an ent field or edge so it is left out of the generated
// protobuf message. [WithAutoFill] does not number skipped members. To omit a
// whole schema, use [SkipGen] instead.
func Skip() schema.Annotation {
	return skipped{}
}

func (f skipped) Name() string {
	return SkipAnnotation
}
