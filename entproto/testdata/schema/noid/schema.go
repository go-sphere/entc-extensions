package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{field.String("name")}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("friends", User.Type).Through("friendships", Friendship.Type),
	}
}

// Friendship is an edge schema with a composite ID, so ent gives it no ID
// field.
type Friendship struct {
	ent.Schema
}

func (Friendship) Annotations() []schema.Annotation {
	return []schema.Annotation{field.ID("user_id", "friend_id")}
}

func (Friendship) Fields() []ent.Field {
	return []ent.Field{field.Int("user_id"), field.Int("friend_id")}
}

func (Friendship) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Unique().Required().Field("user_id"),
		edge.To("friend", User.Type).Unique().Required().Field("friend_id"),
	}
}

// UserView is a view, which has no ID field either.
type UserView struct {
	ent.View
}

func (UserView) Fields() []ent.Field {
	return []ent.Field{field.String("name")}
}
