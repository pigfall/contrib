package entproto

import (
	"fmt"

	"entgo.io/ent"
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

func BuildSchemaIdsStructName(node *gen.Type) string {
	return fmt.Sprintf("%ss",node.ID.StorageKey())
}

func BuildSchemaIdStructName(node *gen.Type) string {
	return node.ID.StorageKey()
}

func FieldAdder() func() int {
	return buildAdderFrom(0)
}

func EdgeAdder(fields []ent.Field) func() int {
	base := FindMaxFieldNum(fields)
	return buildAdderFrom(base)
}

func FindMaxFieldNum(fields []ent.Field) int {
	var maxNum int = 1
	for _, field := range fields {
		for _, annotation := range field.Descriptor().Annotations {
			if annotation.Name() == FieldAnnotation {
				fieldNum := PBFieldNumber(annotation)
				if fieldNum > maxNum {
					maxNum = fieldNum
				}
			}
		}

	}
	return maxNum
}

func buildAdderFrom(base int) func() int {
	var value = base
	return func() int {
		value++
		return value
	}
}
