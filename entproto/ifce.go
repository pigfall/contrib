package entproto

import(
	"entgo.io/ent/entc/gen"
	"google.golang.org/protobuf/types/descriptorpb"
)

type MethodIfce interface{
		GenMethodProtos(
			genType *gen.Type,
			genTypeMsg *descriptorpb.DescriptorProto,
			genTypeMsgId *descriptorpb.DescriptorProto,
		) (methodResources ,error)
}
