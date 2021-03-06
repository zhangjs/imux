package imux

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hkparker/TLJ"
	"net"
	"reflect"
)

// Tag all TLJ sockets as "all"
func tag_socket(socket net.Conn, server *tlj.Server) {
	log.WithFields(log.Fields{
		"at": "tag_socket",
	}).Debug("accepted new socket")
	server.TagSocket(socket, "all")
}

// Create a TLJ type store for only chunks
func type_store() tlj.TypeStore {
	type_store := tlj.NewTypeStore()
	type_store.AddType(
		reflect.TypeOf(Chunk{}),
		reflect.TypeOf(&Chunk{}),
		buildChunk,
	)
	return type_store
}
