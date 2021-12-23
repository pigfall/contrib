package entproto

import (
	"fmt"

	"entgo.io/ent/entc/gen"
	"google.golang.org/protobuf/types/descriptorpb"
)

// build pb field page index
// page_index
func BuildPBPageIndexField() *descriptorpb.FieldDescriptorProto {
	name := strptr("page_index")
	return &descriptorpb.FieldDescriptorProto{
		Name:     name,
		Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
		JsonName: name,
	}
}

// page_size
func BuildPBPageSizeField() *descriptorpb.FieldDescriptorProto {
	name := strptr("page_size")
	return &descriptorpb.FieldDescriptorProto{
		Name:     name,
		Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
		JsonName: name,
	}
}

// {schema}_id
func BuildPBSchemaIdField(node *gen.Type) *descriptorpb.FieldDescriptorProto {
	name := strptr(node.ID.StorageKey())
	return &descriptorpb.FieldDescriptorProto{
		Name:     name,
		Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		JsonName: name,
	}
}

// data_count
func BuildPBDataCountField() *descriptorpb.FieldDescriptorProto {
	name := strptr("data_count")
	return &descriptorpb.FieldDescriptorProto{
		Name:     name,
		Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
		JsonName: name,
	}
}

// repeated {schame}s
func BuildPBSchemaListField(schema *gen.Type) *descriptorpb.FieldDescriptorProto {
	name := strptr(fmt.Sprintf("%ss", schema.Name))
	return &descriptorpb.FieldDescriptorProto{
		Name:     name,
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		TypeName: strptr(schema.Name),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
		JsonName: name,
	}
}
