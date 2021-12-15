package entproto

import(
	strHelper "github.com/iancoleman/strcase"
	"strings"
	"log"
	"text/template"
	"fmt"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	pbHttpOpt  "google.golang.org/genproto/googleapis/api/annotations"
)

type HttpMapping struct{
	Method string
	UrlTpl string
	Summary string
}

type HttpMappingOption func(*HttpMapping) 


func HttpMappingOptionSummary(summary string)HttpMappingOption{
	return func(httpMapping *HttpMapping){
		httpMapping.Summary= summary
	}
}

func HttpMappingNew(httpMethod string,urlTpl string,options ...HttpMappingOption) *HttpMapping{
	httpMapping := &HttpMapping{
		Method:httpMethod,
		UrlTpl:urlTpl,
	}
	for _,option := range options{
		option(httpMapping)
	}
	return httpMapping
}

func MethodOptionHttpOption(httpMapping *HttpMapping)MethodOption{
	return func(method *Method){
		method.HttpMapping = httpMapping
	}
}

func (this *HttpMapping) Visit(method *Method,methodDesc *descriptorpb.MethodDescriptorProto,msgContainer *MsgContainer)error{
	var summary = this.Summary
	if len(summary) == 0 {
		summary = fmt.Sprintf("%s %s",method.Name,msgContainer.genType.Name)
	}
	if methodDesc.Options == nil{
		methodDesc.Options = &descriptorpb.MethodOptions{}
	}

	httpRule := &pbHttpOpt.HttpRule{}
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
		"ToSnake": strHelper.ToSnake,
	}
	urlTplIns,err := template.New("").Funcs(funcMap).Parse(this.UrlTpl)
	if err != nil{
		err= fmt.Errorf("Parse UrlTpl %s failed %w",this.UrlTpl,err)
		log.Println(err.Error())
		return err
	}
	urlTplIns = urlTplIns.Option("missingkey=error")
	urlStrWriter  := &strings.Builder{}
	err = urlTplIns.Execute(urlStrWriter,map[string]interface{}{
		"genType":msgContainer.genType,
		"genTypeId":msgContainer.GetGenTypeIdStorageKey(),
	})
	if err != nil{
		log.Println(err)
		return err
	}
	url := urlStrWriter.String()

	switch this.Method{
	case "post":
		httpRule.Pattern = &pbHttpOpt.HttpRule_Post{
			Post:url,
		}
	case "get":
		httpRule.Pattern = &pbHttpOpt.HttpRule_Get{
			Get:url,
		}
	case "put":
		httpRule.Pattern = &pbHttpOpt.HttpRule_Put{
			Put:url,
		}
	case "patch":
		httpRule.Pattern = &pbHttpOpt.HttpRule_Patch{
			Patch:url,
		}
	case "delete":
		httpRule.Pattern = &pbHttpOpt.HttpRule_Delete{
			Delete:url,
		}
	default:
		err := fmt.Errorf("undefined http method")
		log.Println(err)
		return err
	}

	proto.SetExtension(methodDesc.Options,options.E_Openapiv2Operation,&options.Operation{Summary:summary})
	proto.SetExtension(
		methodDesc.Options,
		pbHttpOpt.E_Http,
		httpRule,
		)
	return nil
}
