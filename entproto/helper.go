package entproto

import (
	"fmt"

	"entgo.io/ent/entc/gen"
	"google.golang.org/protobuf/types/descriptorpb"
)

type MsgContainer struct {
	genType        *gen.Type
	genTypePBMsg   *descriptorpb.DescriptorProto
	genTypePBMsgId *descriptorpb.DescriptorProto
	pageQueryPBMsg *descriptorpb.DescriptorProto
	countPBMsg     *descriptorpb.DescriptorProto
}

type protoPackages struct {
	packages (map[string]*descriptorpb.FileDescriptorProto)
}

func (this *MsgContainer) GetGenTypeIdStorageKey() string {
	return this.genType.ID.StorageKey()
}

func FindSchemaByNameX(nodes []*gen.Type, name string) *gen.Type {
	for _, node := range nodes {
		if node.Name == name {
			return node
		}
	}
	panic(fmt.Errorf("Not find schema %s", name))
}

func BuildSchemaIdStructName(node *gen.Type) string {
	return node.ID.StorageKey()
}
