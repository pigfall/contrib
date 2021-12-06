package entproto

import(
	"entgo.io/ent/entc/gen"
	"google.golang.org/protobuf/types/descriptorpb"
)

type MsgContainer struct{
	genType *gen.Type
	genTypePBMsg *descriptorpb.DescriptorProto
	genTypePBMsgId *descriptorpb.DescriptorProto
	pageQueryPBMsg *descriptorpb.DescriptorProto
	countPBMsg *descriptorpb.DescriptorProto
}

type  protoPackages struct{
	packages (map[string]*descriptorpb.FileDescriptorProto)
}


