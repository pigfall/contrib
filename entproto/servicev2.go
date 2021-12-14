package entproto

import (
	"fmt"
	"log"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	ServiceV2Annotation = "ProtoServiceV2"
)

type servicev2 struct {
	Methods []*Method
}

func ServiceV2(methods ...*Method) schema.Annotation {
	return &servicev2{
		Methods: methods,
	}
}

func (servicev2) Name() string {
	return ServiceV2Annotation
}

func extractServiceV2Annotation(sch *gen.Type) (*servicev2, error) {
	annot, ok := sch.Annotations[ServiceV2Annotation]
	if !ok {
		return nil, fmt.Errorf("%w: entproto: schema %q does not have an entproto.ServiceV2 annotation",
			errNoServiceDef, sch.Name)
	}

	var out servicev2
	err := mapstructure.Decode(annot, &out)
	if err != nil {
		return nil, fmt.Errorf("entproto: unable to decode entproto.ServiceV2 annotation for schema %q: %w",
			sch.Name, err)
	}

	return &out, nil
}

func (this servicev2) createServiceResources(adaptor *Adapter, pkgName string, msgContainer *MsgContainer) (serviceResources, error) {
	genType := msgContainer.genType
	genTypeMsg := msgContainer.genTypePBMsg
	name := genType.Name
	serviceFqn := fmt.Sprintf("%sService", name)

	out := serviceResources{
		svc: &descriptorpb.ServiceDescriptorProto{
			Name: &serviceFqn,
		},
	}

	// < add repeated type
	genTypeMsgsList := &descriptorpb.DescriptorProto{
		Name:     strptr(fmt.Sprintf("%ss", genType.Name)),
		EnumType: []*descriptorpb.EnumDescriptorProto(nil),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     strptr("data_list"),
				Number:   int32ptr(1),
				TypeName: strptr(genTypeMsg.GetName()),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			},
		},
	}
	err := adaptor.AddMessageDescriptorNoExtractDep(
		pkgName,
		genTypeMsgsList,
	)
	if err != nil {
		log.Println(err)
		return out, err
	}
	// >

	for _, m := range this.Methods {
		resources, err := m.genMethodProtos(pkgName, msgContainer, genTypeMsgsList)
		if err != nil {
			return serviceResources{}, err
		}
		out.svc.Method = append(out.svc.Method, resources.methodDescriptor)
		out.svcMessages = append(out.svcMessages, resources.input)
	}

	// < generate releated edge rpc method
	node := msgContainer.genType
	for _, edge := range node.Edges {
		twoTypeIdStructName := fmt.Sprintf("%sIdAnd%sId", genType.Name, edge.Type.Name)
		twoTypeStruct := &descriptorpb.DescriptorProto{
			Name: strptr(twoTypeIdStructName),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name: strptr(BuildSchemaIdStructName(genType)),
					Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
				{
					Name: strptr(BuildSchemaIdStructName(edge.Type)),
					Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
			},
		}
		adaptor.AddMessageDescriptorNoExtractDep(pkgName, twoTypeStruct)
		if (edge.Rel.Type == gen.O2O) || (edge.Rel.Type == gen.O2M) {
			out.svc.Method = append(
				out.svc.Method,
				&descriptorpb.MethodDescriptorProto{
					Name:       strptr(fmt.Sprintf("Add%s", edge.Type.Name)),
					InputType:  strptr(edge.Type.Name),
					OutputType: strptr(fmt.Sprintf("%sId", edge.Type.Name)),
				},
			)

			out.svc.Method = append(
				out.svc.Method,
				&descriptorpb.MethodDescriptorProto{
					Name:       strptr(fmt.Sprintf("Add%sById", edge.Type.Name)),
					InputType:  strptr(twoTypeIdStructName),
					OutputType: strptr("google.protobuf.Empty"),
				},
			)

			out.svc.Method = append(
				out.svc.Method,
				&descriptorpb.MethodDescriptorProto{
					Name:       strptr(fmt.Sprintf("Remove%s", edge.Type.Name)),
					InputType:  strptr(twoTypeIdStructName),
					OutputType: strptr("google.protobuf.Empty"),
				},
			)

		} else if edge.Rel.Type == gen.M2M {
			edgeNode := FindSchemaByNameX(adaptor.graph.Nodes, edge.Type.Name)
			edgePBDesc, err := adaptor.toProtoMessageDescriptor(edgeNode)
			if err != nil {
				log.Println((err))
				return out, err
			}
			reqName := fmt.Sprintf("Add%sReq", edge.Type.Name)
			edgePBDesc.Name = strptr(reqName)
			edgePBDesc.Field = append(edgePBDesc.Field, &descriptorpb.FieldDescriptorProto{
				Name: strptr(genType.ID.StorageKey()),
			})
			// TODO
			adaptor.AddMessageDescriptorNoExtractDep(pkgName, edgePBDesc)

			out.svc.Method = append(
				out.svc.Method,
				&descriptorpb.MethodDescriptorProto{
					Name:       strptr(fmt.Sprintf("Add%s", edge.Type.Name)),
					InputType:  strptr(reqName),
					OutputType: strptr(fmt.Sprintf("%sId", edge.Type.Name)),
				},
			)

			out.svc.Method = append(
				out.svc.Method,
				&descriptorpb.MethodDescriptorProto{
					Name:       strptr(fmt.Sprintf("Add%sById", edge.Type.Name)),
					InputType:  strptr(twoTypeIdStructName),
					OutputType: strptr("google.protobuf.Empty"),
				},
			)

			out.svc.Method = append(
				out.svc.Method,
				&descriptorpb.MethodDescriptorProto{
					Name:       strptr(fmt.Sprintf("Remove%s", edge.Type.Name)),
					InputType:  strptr(twoTypeIdStructName),
					OutputType: strptr("google.protobuf.Empty"),
				},
			)
		}
	}
	// >

	return out, nil

}
