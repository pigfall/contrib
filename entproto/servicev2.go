package entproto

import(
	"log"
	"google.golang.org/protobuf/types/descriptorpb"
	"github.com/mitchellh/mapstructure"
	"fmt"
	"entgo.io/ent/schema"
	"entgo.io/ent/entc/gen"
)

const(
		ServiceV2Annotation = "ProtoServiceV2"
)

type servicev2 struct{
	Methods []*Method
}

func ServiceV2(methods  ... *Method)schema.Annotation{
	return &servicev2{
		Methods:methods,
	}
}

func (servicev2) Name() string{
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


func (this servicev2)  createServiceResources(adaptor *Adapter,pkgName string,genType *gen.Type,genTypeMsg *descriptorpb.DescriptorProto,genTypeMsgId *descriptorpb.DescriptorProto)(serviceResources, error){
	name := genType.Name
	serviceFqn := fmt.Sprintf("%sService", name)

	out := serviceResources{
		svc: &descriptorpb.ServiceDescriptorProto{
			Name: &serviceFqn,
		},
	}

	// < add repeated type
	genTypeMsgsList :=&descriptorpb.DescriptorProto{
		Name:strptr(fmt.Sprintf("%ss",genType.Name)),
		EnumType: []*descriptorpb.EnumDescriptorProto(nil),
		Field:[]*descriptorpb.FieldDescriptorProto{
			{
				Name:strptr("data_list"),
				Number: int32ptr(1),
				TypeName:strptr(genTypeMsg.GetName()),
				Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			},
		},
	}
	err := adaptor.AddMessageDescriptorNoExtractDep(
		pkgName,
		genTypeMsgsList,
	)
	if err != nil{
		log.Println(err)
		return out,err
	}
	// >

	for _, m := range this.Methods {
		resources, err := m.genMethodProtos(pkgName,&MsgContainer{
			genType :genType,
			genTypePBMsg :genTypeMsg,
			genTypePBMsgId :genTypeMsgId,
		},genTypeMsgsList)
		if err != nil {
			return serviceResources{}, err
		}
		out.svc.Method = append(out.svc.Method, resources.methodDescriptor)
		out.svcMessages = append(out.svcMessages, resources.input)
	}

	return out, nil

}
