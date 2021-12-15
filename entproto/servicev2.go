package entproto

import (
	"fmt"
	"log"
	"strings"

	tpl "text/template"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema"
	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	strHelper "github.com/iancoleman/strcase"
	"github.com/mitchellh/mapstructure"
	pbHttpOpt "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
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
			methodEdgeAdd := &descriptorpb.MethodDescriptorProto{
				Name:       strptr(fmt.Sprintf("Add%s", edge.Type.Name)),
				InputType:  strptr(edge.Type.Name),
				OutputType: strptr(fmt.Sprintf("%sId", edge.Type.Name)),
				Options:    &descriptorpb.MethodOptions{},
			}
			httpRule := &pbHttpOpt.HttpRule{}
			url, err := edgeAddUrlTpl(genType, edge)
			if err != nil {
				log.Println(err)
				return out, err
			}
			httpRule.Pattern = &pbHttpOpt.HttpRule_Post{
				Post: url,
			}
			out.svc.Method = append(
				out.svc.Method,
				methodEdgeAdd,
			)

			proto.SetExtension(methodEdgeAdd.Options, options.E_Openapiv2Operation, &options.Operation{Summary: ""})
			proto.SetExtension(
				methodEdgeAdd.Options,
				pbHttpOpt.E_Http,
				httpRule,
			)

			httpRule = &pbHttpOpt.HttpRule{}

			url, err = nodeIdAndEdgeIdUrlTpl(genType, edge)
			if err != nil {
				log.Println(err)
				return out, err
			}
			httpRule.Pattern = &pbHttpOpt.HttpRule_Post{
				Post: url,
			}

			methodEdgeAddById := &descriptorpb.MethodDescriptorProto{
				Name:       strptr(fmt.Sprintf("Add%sById", edge.Type.Name)),
				InputType:  strptr(twoTypeIdStructName),
				OutputType: strptr("google.protobuf.Empty"),
				Options:    &descriptorpb.MethodOptions{},
			}
			out.svc.Method = append(
				out.svc.Method,
				methodEdgeAddById,
			)

			proto.SetExtension(methodEdgeAddById.Options, options.E_Openapiv2Operation, &options.Operation{Summary: ""})
			proto.SetExtension(
				methodEdgeAddById.Options,
				pbHttpOpt.E_Http,
				httpRule,
			)

			methodEdgeRemove := &descriptorpb.MethodDescriptorProto{
				Name:       strptr(fmt.Sprintf("Remove%s", edge.Type.Name)),
				InputType:  strptr(twoTypeIdStructName),
				OutputType: strptr("google.protobuf.Empty"),
				Options:    &descriptorpb.MethodOptions{},
			}

			out.svc.Method = append(
				out.svc.Method,
				methodEdgeRemove,
			)

			proto.SetExtension(methodEdgeRemove.Options, options.E_Openapiv2Operation, &options.Operation{Summary: ""})
			proto.SetExtension(
				methodEdgeRemove.Options,
				pbHttpOpt.E_Http,
				httpRule,
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
				Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
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

func edgeAddUrlTpl(node *gen.Type, edge *gen.Edge) (string, error) {
	tplIns, err := tpl.New("").Parse("/{{.nodeName}}s/{ {{- .nodeIdStorageKey -}} }/{{.edgeTypeName}}s")
	if err != nil {
		log.Println(err)
		return "", err
	}
	sb := strings.Builder{}
	err = tplIns.Execute(&sb, map[string]interface{}{
		"nodeName":         strHelper.ToSnake(node.Name),
		"nodeIdStorageKey": node.ID.StorageKey(),
		"edgeTypeName":     strHelper.ToSnake(edge.Type.Name),
	})
	if err != nil {
		log.Println(err)
		return "", err
	}
	return sb.String(), nil
}

func nodeIdAndEdgeIdUrlTpl(node *gen.Type, edge *gen.Edge) (string, error) {
	tplIns, err := tpl.New("").Parse("/{{.nodeName}}s/{ {{- .nodeIdStorageKey -}} }/{{.edgeTypeName}}s/{ {{- .edgeTypeIdStorageKey -}} }")
	if err != nil {
		log.Println(err)
		return "", err
	}
	sb := strings.Builder{}
	err = tplIns.Execute(&sb, map[string]interface{}{
		"nodeName":             strHelper.ToSnake(node.Name),
		"nodeIdStorageKey":     node.ID.StorageKey(),
		"edgeTypeName":         strHelper.ToSnake(edge.Type.Name),
		"edgeTypeIdStorageKey": edge.Type.ID.StorageKey(),
	})
	if err != nil {
		log.Println(err)
		return "", err
	}
	return sb.String(), nil
}
