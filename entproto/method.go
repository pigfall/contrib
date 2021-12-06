package entproto

import(
	"log"
		"fmt"
	"google.golang.org/protobuf/types/descriptorpb"
)

type MethodInOutType int

const(
	MethodInOutType_UnKnown MethodInOutType = iota
	MethodInOutType_GenType 
	MethodInOutType_GenTypeId
	MethodInOutType_Empty 
	MethodInOutType_PageQuery
	MethodInOutType_GenTypes
	MethodInOutType_Count
)

type Method struct {
	Name string
	InputType MethodInOutType
	OutputType MethodInOutType
	HttpMapping *HttpMapping
	Summary string
}

type MethodOption func(*Method)


func NewMethod(name string,inputType MethodInOutType,outputType MethodInOutType,options ...MethodOption)*Method{
	m := &Method {
		InputType:inputType,
		OutputType:outputType,
		Name:name,
	}
	for _,option := range options{
		option(m)
	}
	return m
}

func (this Method) inOutTypeToPBMsg(protoPkgName string,msgContainer *MsgContainer,methodInOutType MethodInOutType,repeatedMsg *descriptorpb.DescriptorProto)(*descriptorpb.DescriptorProto,string,error){
	switch  methodInOutType{
	case MethodInOutType_GenType:
		return msgContainer.genTypePBMsg,msgContainer.genTypePBMsg.GetName(),nil
	case MethodInOutType_GenTypeId:
		return msgContainer.genTypePBMsgId,msgContainer.genTypePBMsgId.GetName(),nil
	case MethodInOutType_Empty:
		return nil,"google.protobuf.Empty",nil
	case MethodInOutType_GenTypes:
		return repeatedMsg,repeatedMsg.GetName(),nil
	case MethodInOutType_PageQuery:
		return msgContainer.pageQueryPBMsg,msgContainer.pageQueryPBMsg.GetName(),nil
		case MethodInOutType_Count:
		return msgContainer.countPBMsg,msgContainer.countPBMsg.GetName(),nil
	default:
		err :=  fmt.Errorf("Undefined MethodInputType %v",this.InputType)
		log.Println(err)
		return nil,"",err
	}
}


func (this Method)genMethodProtos(protoPkgName string,msgContainer *MsgContainer,repeatedMsg *descriptorpb.DescriptorProto) (methodResources, error) {
	inputType,inputTypeName,err := this.inOutTypeToPBMsg(protoPkgName,msgContainer,this.InputType,repeatedMsg)
	if err != nil{
		log.Println(err)
		return methodResources{},err
	}
	_,outputTypeName,err := this.inOutTypeToPBMsg(protoPkgName,msgContainer,this.OutputType,repeatedMsg)
	if err != nil{
		log.Println(err)
		return methodResources{},err
	}
	methodDesc := &descriptorpb.MethodDescriptorProto{
		Name:       strptr(this.Name),
		InputType:  strptr(inputTypeName),
		OutputType: strptr(outputTypeName),
	}

	if this.HttpMapping != nil{
		err = this.HttpMapping.Visit(&this,methodDesc,msgContainer)
		if err != nil{
			log.Println(err)
			return methodResources{},err
		}
	}
	return methodResources{
		methodDescriptor :methodDesc,
		input            :inputType,
	},nil
}
