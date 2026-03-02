package entproto

import (
	"path/filepath"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"github.com/stretchr/testify/require"
)

func TestLoadAdapter(t *testing.T) {
	graph, err := entc.LoadGraph("./internal/todo/ent/schema", &gen.Config{})
	require.NoError(t, err)

	adapter, err := LoadAdapter(graph)
	require.NoError(t, err)

	// Test User message
	fd, err := adapter.GetFileDescriptor("User")
	require.NoError(t, err)
	require.Equal(t, filepath.Join("entpb", "entpb.proto"), fd.GetName())

	msg := fd.FindMessage("entpb.User")
	require.NotNil(t, msg)

	idField := msg.FindFieldByName("id")
	require.NotNil(t, idField)
	require.EqualValues(t, 1, idField.GetNumber())

	nameField := msg.FindFieldByName("name")
	require.NotNil(t, nameField)
	require.EqualValues(t, 2, nameField.GetNumber())
	require.Equal(t, "TYPE_STRING", nameField.GetType().String())

	ageField := msg.FindFieldByName("age")
	require.NotNil(t, ageField)
	require.EqualValues(t, 3, ageField.GetNumber())

	activeField := msg.FindFieldByName("active")
	require.NotNil(t, activeField)
	require.EqualValues(t, 4, activeField.GetNumber())

	// Test Post message
	fd, err = adapter.GetFileDescriptor("Post")
	require.NoError(t, err)

	msg = fd.FindMessage("entpb.Post")
	require.NotNil(t, msg)

	titleField := msg.FindFieldByName("title")
	require.NotNil(t, titleField)
	require.EqualValues(t, 2, titleField.GetNumber())

	authorField := msg.FindFieldByName("author")
	require.NotNil(t, authorField)
	require.EqualValues(t, 4, authorField.GetNumber())

	// Test Task message (enum)
	fd, err = adapter.GetFileDescriptor("Task")
	require.NoError(t, err)

	msg = fd.FindMessage("entpb.Task")
	require.NotNil(t, msg)

	statusField := msg.FindFieldByName("status")
	require.NotNil(t, statusField)
	require.EqualValues(t, 3, statusField.GetNumber())
	require.Equal(t, "TYPE_ENUM", statusField.GetType().String())

	enumType := statusField.GetEnumType()
	require.NotNil(t, enumType)
	require.Equal(t, "entpb.Task.Status", enumType.GetFullyQualifiedName())
}

func TestSkipMessage(t *testing.T) {
	graph, err := entc.LoadGraph("./internal/todo/ent/schema", &gen.Config{})
	require.NoError(t, err)

	adapter, err := LoadAdapter(graph)
	require.NoError(t, err)

	// Test that edges are converted to fields
	fd, err := adapter.GetFileDescriptor("User")
	require.NoError(t, err)

	msg := fd.FindMessage("entpb.User")
	require.NotNil(t, msg)

	// Check edge field
	postsField := msg.FindFieldByName("posts")
	require.NotNil(t, postsField)
	require.EqualValues(t, 5, postsField.GetNumber())
	require.Equal(t, "TYPE_MESSAGE", postsField.GetType().String())
}
