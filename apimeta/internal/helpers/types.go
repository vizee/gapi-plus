package helpers

import (
	"github.com/vizee/gapi/metadata"
	"github.com/vizee/jsonpb"
)

type Message struct {
	*jsonpb.Message
	Bindings   []metadata.FieldBinding
	MapEntry   bool
	Incomplete bool
}
