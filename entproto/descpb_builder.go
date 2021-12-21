package entproto

import (
	"fmt"

	"entgo.io/ent/entc/gen"
	"google.golang.org/protobuf/types/descriptorpb"
)

// build pb field page index
// page_index
func BuildPBPageIndexField() *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strptr("page_index"),
		Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
	}
}

// page_size
func BuildPBPageSizeField() *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strptr("page_size"),
		Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
	}
}

// {schema}_id
func BuildPBSchemaIdField(node *gen.Type) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strptr(node.ID.StorageKey()),
		Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}
}

// data_count
func BuildPBDataCountField() *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strptr("data_count"),
		Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
	}
}

// repeated {schame}s
func BuildPBSchemaListField(schema *gen.Type) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     strptr(fmt.Sprintf("%ss", schema.Name)),
		TypeName: strptr(schema.Name),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
	}
}
