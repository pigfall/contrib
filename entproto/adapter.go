// Copyright 2019-present Facebook
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entproto

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/builder"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	_ "google.golang.org/protobuf/types/known/emptypb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	_ "google.golang.org/protobuf/types/known/wrapperspb" // needed to load wkt to global proto registry
)

const (
	DefaultProtoPackageName = "entpb"
	IDFieldNumber           = 1
)

var (
	ErrSchemaSkipped   = errors.New("entproto: schema not annotated with Generate=true")
	repeatedFieldLabel = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	wktsPaths          = map[string]string{
		// TODO: handle more Well-Known proto types
		"google.protobuf.Timestamp":   "google/protobuf/timestamp.proto",
		"google.protobuf.Empty":       "google/protobuf/empty.proto",
		"google.protobuf.Int32Value":  "google/protobuf/wrappers.proto",
		"google.protobuf.Int64Value":  "google/protobuf/wrappers.proto",
		"google.protobuf.UInt32Value": "google/protobuf/wrappers.proto",
		"google.protobuf.UInt64Value": "google/protobuf/wrappers.proto",
		"google.protobuf.FloatValue":  "google/protobuf/wrappers.proto",
		"google.protobuf.DoubleValue": "google/protobuf/wrappers.proto",
		"google.protobuf.StringValue": "google/protobuf/wrappers.proto",
		"google.protobuf.BoolValue":   "google/protobuf/wrappers.proto",
		"google.protobuf.BytesValue":  "google/protobuf/wrappers.proto",
	}
)

// LoadAdapter takes a *gen.Graph and parses it into protobuf file descriptors
func LoadAdapter(graph *gen.Graph) (*Adapter, error) {
	a := &Adapter{
		graph:            graph,
		descriptors:      make(map[string]*desc.FileDescriptor),
		schemaProtoFiles: make(map[string]string),
		errors:           make(map[string]error),
	}
	if err := a.parse(); err != nil {
		return nil, err
	}
	return a, nil
}

// Adapter facilitates the transformation of ent gen.Type to desc.FileDescriptors
type Adapter struct {
	graph            *gen.Graph
	descriptors      map[string]*desc.FileDescriptor
	schemaProtoFiles map[string]string
	errors           map[string]error
	protoPackages    map[string]*descriptorpb.FileDescriptorProto
}

// AllFileDescriptors returns a file descriptor per proto package for each package that contains
// a successfully parsed ent.Schema
func (a *Adapter) AllFileDescriptors() map[string]*desc.FileDescriptor {
	return a.descriptors
}

// GetMessageDescriptor retrieves the protobuf message descriptor for `schemaName`, if an error was returned
// while trying to parse that error they are returned
func (a *Adapter) GetMessageDescriptor(schemaName string) (*desc.MessageDescriptor, error) {
	fd, err := a.GetFileDescriptor(schemaName)
	if err != nil {
		return nil, err
	}
	findMessage := fd.FindMessage(fd.GetPackage() + "." + schemaName)
	if findMessage != nil {
		return findMessage, nil
	}
	return nil, errors.New("entproto: couldnt find message descriptor")
}
func (a *Adapter) AddMessageDescriptorNoExtractDep(packageName string, messageDescriptor *descriptorpb.DescriptorProto) error {
	return a.addMessageDescriptor(packageName, messageDescriptor, false)
}
func (a *Adapter) addMessageDescriptor(packageName string, messageDescriptor *descriptorpb.DescriptorProto, extractDep bool) error {
	protoPkg := packageName

	if _, ok := a.protoPackages[protoPkg]; !ok {
		goPkg := a.goPackageName(protoPkg)

		// < custome go option package
		optionGoPackage, exist := os.LookupEnv("OPTION_GO_PACKAGE")
		if exist {
			goPkg = optionGoPackage
		}
		// >

		a.protoPackages[protoPkg] = &descriptorpb.FileDescriptorProto{
			Name:    relFileName(protoPkg),
			Package: &protoPkg,
			Syntax:  strptr("proto3"),
			Options: &descriptorpb.FileOptions{
				GoPackage: &goPkg,
			},
		}
	}
	fd := a.protoPackages[protoPkg]
	for _, v := range fd.MessageType {
		if v.GetName() == messageDescriptor.GetName() {
			return nil
		}
	}
	fd.MessageType = append(fd.MessageType, messageDescriptor)

	if extractDep {
		depPaths, err := a.extractDepPaths(messageDescriptor)
		if err != nil {
			log.Println(err)
			return err
		}
		fd.Dependency = append(fd.Dependency, depPaths...)
		return nil
	}
	return nil
}

func (a *Adapter) AddMessageDescriptor(packageName string, messageDescriptor *descriptorpb.DescriptorProto) error {
	return a.addMessageDescriptor(packageName, messageDescriptor, true)
}

// parse transforms the ent gen.Type objects into file descriptors
func (a *Adapter) parse() error {
	var dpbDescriptors []*descriptorpb.FileDescriptorProto

	a.protoPackages = make(map[string]*descriptorpb.FileDescriptorProto)
	protoPackages := a.protoPackages

	msgContaienrs := make([]*MsgContainer, 0)

	for _, genType := range a.graph.Nodes {
		messageDescriptor, err := a.toProtoMessageDescriptor(genType)

		// store specific message parse failures
		if err != nil {
			a.errors[genType.Name] = err
			log.Println(err)
			continue
		}

		protoPkg, err := protoPackageName(genType)
		if err != nil {
			a.errors[genType.Name] = err
			log.Println(err)
			continue
		}
		err = a.AddMessageDescriptor(protoPkg, messageDescriptor)
		if err != nil {
			log.Println(err)
			return err
		}

		fd := a.protoPackages[protoPkg]

		a.schemaProtoFiles[genType.Name] = *fd.Name

		msgContaienrs = append(msgContaienrs, &MsgContainer{genType: genType, genTypePBMsg: messageDescriptor})

		svcAnnotation, err := extractServiceAnnotation(genType)
		if errors.Is(err, errNoServiceDef) {
			continue
		}
		if err != nil {
			log.Println(err)
			return err
		}
		if svcAnnotation.Generate {
			svcResources, err := a.createServiceResources(genType)
			if err != nil {
				log.Println(err)
				return err
			}
			fd.Service = append(fd.Service, svcResources.svc)
			fd.MessageType = append(fd.MessageType, svcResources.svcMessages...)
			fd.Dependency = append(fd.Dependency, "google/protobuf/empty.proto")
		}

	}

	// < add message MsgId
	for _, msgContainer := range msgContaienrs {
		genType := msgContainer.genType
		pbMsgId := &descriptorpb.DescriptorProto{
			Name:     strptr(fmt.Sprintf("%sId", msgContainer.genType.Name)),
			EnumType: []*descriptorpb.EnumDescriptorProto(nil),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     strptr(genType.ID.StorageKey()),
					Number:   int32ptr(1),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					JsonName: (strptr(genType.ID.StorageKey())),
				},
			},
		}
		protoPkg, err := protoPackageName(genType)
		if err != nil {
			log.Println(err)
			return err
		}
		err = a.AddMessageDescriptor(protoPkg, pbMsgId)
		if err != nil {
			log.Println(err)
			return err
		}
		msgContainer.genTypePBMsgId = pbMsgId
	}
	// >

	// < generate servicev2
	for _, msgContainer := range msgContaienrs {
		genType := msgContainer.genType
		svcV2, err := extractServiceV2Annotation(genType)
		if errors.Is(err, errNoServiceDef) {
			continue
		}
		if err != nil {
			log.Println(err)
			return err
		}
		protoPkg, err := protoPackageName(genType)
		if err != nil {
			log.Println(err)
			return err
		}
		countPBMsg := &descriptorpb.DescriptorProto{
			Name: strptr("CountNumber"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name: strptr("value"),
					Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
				},
			},
		}
		err = a.AddMessageDescriptorNoExtractDep(protoPkg, countPBMsg)
		if err != nil {
			log.Println(err)
			return err
		}
		msgContainer.countPBMsg = countPBMsg

		// < generate pageFind entity
		genTypePBMsgCopy, err := a.toProtoMessageDescriptor(msgContainer.genType)
		if err != nil {
			log.Println(err)
			return err
		}
		// change name
		genTypePBMsgCopy.Name = strptr(fmt.Sprintf("%sPageQuery", genTypePBMsgCopy.GetName()))
		// add page query field
		genTypePBMsgCopy.Field = append(
			genTypePBMsgCopy.Field,
			[]*descriptorpb.FieldDescriptorProto{
				BuildPBPageIndexField(),
				BuildPBPageSizeField(),
				BuildPBPageRecordCount(),
			}...,
		)
		var pageQueryPBMsg = genTypePBMsgCopy
		msgContainer.pageQueryPBMsg = pageQueryPBMsg
		// >

		svcResources, err := svcV2.createServiceResources(
			a,
			protoPkg,
			msgContainer,
		)
		if err != nil {
			log.Println(err)
			return err
		}
		fd := protoPackages[protoPkg]
		fd.Service = append(fd.Service, svcResources.svc)
		for _, v := range svcResources.svcMessages {
			err = a.AddMessageDescriptor(protoPkg, v)
			if err != nil {
				log.Println(err)
				return err
			}
		}
		fd.Dependency = append(fd.Dependency, "google/protobuf/empty.proto")
	}
	// >

	//
	//

	// Append the well known types to the context.
	for _, wktPath := range wktsPaths {
		typeDesc, err := desc.LoadFileDescriptor(wktPath)
		if err != nil {
			log.Println(err)
			return err
		}
		dpbDescriptors = append(dpbDescriptors, typeDesc.AsFileDescriptorProto())
	}

	for _, fd := range protoPackages {
		fd.Dependency = dedupe(fd.Dependency)
		dpbDescriptors = append(dpbDescriptors, fd)
	}

	descriptors, err := desc.CreateFileDescriptors(dpbDescriptors)
	if err != nil {
		log.Println(err)
		return err
	}

	// cleanup the WKT protos from the map
	for _, wp := range wktsPaths {
		delete(descriptors, wp)
	}

	for dp, fd := range descriptors {
		fbuild, err := builder.FromFile(fd)
		if err != nil {
			log.Println(err)
			return err
		}
		fbuild.SetSyntaxComments(builder.Comments{
			LeadingComment: " Code generated by entproto. DO NOT EDIT.",
		})
		fd, err = fbuild.Build()
		if err != nil {
			log.Println(err)
			return err
		}
		descriptors[dp] = fd
	}

	a.descriptors = descriptors

	return nil
}

func (a *Adapter) goPackageName(protoPkgName string) string {
	// TODO(rotemtam): make this configurable from an annotation
	entBase := a.graph.Config.Package
	slashed := strings.ReplaceAll(protoPkgName, ".", "/")
	return path.Join(entBase, "proto", slashed)
}

// GetFileDescriptor returns the proto file descriptor containing the transformed proto message descriptor for
// `schemaName` along with any other messages in the same protobuf package.
func (a *Adapter) GetFileDescriptor(schemaName string) (*desc.FileDescriptor, error) {
	if err, ok := a.errors[schemaName]; ok {
		return nil, err
	}
	fn, ok := a.schemaProtoFiles[schemaName]
	if !ok {
		return nil, fmt.Errorf("entproto: could not find file descriptor for schema %s", schemaName)
	}

	dsc, ok := a.descriptors[fn]
	if !ok {
		return nil, fmt.Errorf("entproto: could not find file descriptor for schema %s", schemaName)
	}

	return dsc, nil
}

func protoPackageName(genType *gen.Type) (string, error) {
	msgAnnot, err := extractMessageAnnotation(genType)
	if err != nil {
		return "", err
	}

	if msgAnnot.Package != "" {
		return msgAnnot.Package, nil
	}
	return DefaultProtoPackageName, nil
}

func relFileName(packageName string) *string {
	parts := strings.Split(packageName, ".")
	fileName := parts[len(parts)-1] + ".proto"
	parts = append(parts, fileName)
	joined := filepath.Join(parts...)
	return &joined
}

func (a *Adapter) extractDepPaths(m *descriptorpb.DescriptorProto) ([]string, error) {
	var out []string
	for _, fld := range m.Field {
		if fld.Type != nil {
			if *fld.Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE { //nolint
				fieldTypeName := *fld.TypeName
				if wp, ok := wktsPaths[fieldTypeName]; ok { //nolint
					out = append(out, wp)
				} else if graphContainsDependency(a.graph, fieldTypeName) {
					fieldTypeName = extractLastFqnPart(fieldTypeName)
					depType, err := extractGenTypeByName(a.graph, fieldTypeName)
					if err != nil {
						log.Println(err)
						return nil, err
					}
					depPackageName, err := protoPackageName(depType)
					if err != nil {
						log.Println(err)
						return nil, err
					}
					selfType, err := extractGenTypeByName(a.graph, *m.Name)
					if err != nil {
						log.Printf("%+v\n", m)
						log.Println(fieldTypeName)
						log.Println(err)
						return nil, err
					}
					selfPackageName, _ := protoPackageName(selfType)
					if depPackageName != selfPackageName {
						importPath := relFileName(depPackageName)
						out = append(out, *importPath)
					}
				} else {
					//return nil, fmt.Errorf("entproto: failed extracting deps, unknown path for %s", fieldTypeName)
					// TODO
					return nil, nil
				}
			}
		}

	}
	return out, nil
}

func graphContainsDependency(graph *gen.Graph, fieldTypeName string) bool {
	gt, err := extractGenTypeByName(graph, extractLastFqnPart(fieldTypeName))
	if err != nil {
		return false
	}
	return gt != nil
}

func extractLastFqnPart(fqn string) string {
	parts := strings.Split(fqn, ".")
	return parts[len(parts)-1]
}

type unsupportedTypeError struct {
	Type *field.TypeInfo
}

func (e unsupportedTypeError) Error() string {
	return fmt.Sprintf("unsupported field type %q", e.Type.ConstName())
}

func (a *Adapter) toProtoMessageDescriptor(genType *gen.Type) (*descriptorpb.DescriptorProto, error) {
	msgAnnot, err := extractMessageAnnotation(genType)
	if err != nil || !msgAnnot.Generate {
		return nil, ErrSchemaSkipped
	}
	msg := &descriptorpb.DescriptorProto{
		Name:     &genType.Name,
		EnumType: []*descriptorpb.EnumDescriptorProto(nil),
	}

	if !genType.ID.UserDefined {
		// TODO
		// genType.ID.Annotations = map[string]interface{}{FieldAnnotation: Field(IDFieldNumber)}
	}

	all := []*gen.Field{genType.ID}
	all = append(all, genType.Fields...)

	for _, f := range all {
		protoField, err := toProtoFieldDescriptor(f)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		// <
		if protoField == nil {
			// ignore this field
			continue
		}
		// >
		// If the field is an enum type, we need to create the enum descriptor as well.
		if f.Type.Type == field.TypeEnum {
			dp, err := toProtoEnumDescriptor(f)
			if err != nil {
				return nil, err
			}
			msg.EnumType = append(msg.EnumType, dp)
		}
		msg.Field = append(msg.Field, protoField)
	}

	for _, e := range genType.Edges {
		descriptor, err := a.extractEdgeFieldDescriptor(genType, e)
		if err != nil {
			return nil, err
		}
		if descriptor != nil {
			msg.Field = append(msg.Field, descriptor)
		}
	}

	if err := verifyNoDuplicateFieldNumbers(msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func (a *Adapter) toProtoMessageDescriptorCountReq(genType *gen.Type) (*descriptorpb.DescriptorProto, error) {
	msg, err := a.toProtoMessageDescriptor(genType)
	if err != nil {
		return nil, err
	}
	var msgName = fmt.Sprintf("%sCountReq", msg.GetName())
	msg.Name = &msgName
	return msg, nil
}

func (a *Adapter) toProtoMessageDescriptorPageQuery(genType *gen.Type, msg *descriptorpb.DescriptorProto) (*descriptorpb.DescriptorProto, error) {
	rawMsg, err := a.toProtoMessageDescriptor(genType)
	if err != nil {
		return nil, err
	}

	msgName := fmt.Sprintf("%sPageQuery", *rawMsg.Name)
	rawMsg.Name = &msgName

	var maxFieldNumber int32 = 1
	for _, field := range rawMsg.Field {
		if (*field.Number) > maxFieldNumber {
			maxFieldNumber = (*field.Number)
		}
	}

	var pageIndex = "pageIndex"
	var pageSize = "pageSize"

	for i, fldName := range []string{pageIndex, pageSize} {
		fldNum := maxFieldNumber + int32(1+i)
		addFld := func(fldNum int32, fldName string) {
			rawMsg.Field = append(rawMsg.Field, &descriptorpb.FieldDescriptorProto{
				Name:   &fldName,
				Number: &(fldNum),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
			})
		}
		addFld(fldNum, fldName)
	}

	return rawMsg, nil
}

func verifyNoDuplicateFieldNumbers(msg *descriptorpb.DescriptorProto) error {
	mem := make(map[int32]struct{})
	for _, fld := range msg.Field {
		if _, seen := mem[fld.GetNumber()]; seen {
			return fmt.Errorf("entproto: field %d already defined on message %q",
				fld.GetNumber(), msg.GetName())
		} else {
			mem[fld.GetNumber()] = struct{}{}
		}
	}
	return nil
}

func (a *Adapter) extractEdgeFieldDescriptor(source *gen.Type, e *gen.Edge) (*descriptorpb.FieldDescriptorProto, error) {
	t := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	msgTypeName := pascal(e.Type.Name)

	edgeAnnotation, err := extractEdgeAnnotation(e)
	if err != nil {
		return nil, fmt.Errorf("entproto: failed extracting proto field number annotation: %w", err)
	}
	// < ignore
	if edgeAnnotation == nil {
		return nil, nil
	}
	// >

	if edgeAnnotation.Number == 1 {
		return nil, fmt.Errorf("entproto: edge %q has number 1 which is reserved for id", e.Name)
	}

	if edgeAnnotation.Type != descriptorpb.FieldDescriptorProto_Type(0) {
		t = edgeAnnotation.Type
	}

	// < edge name
	var edgeName = e.Name
	if len(edgeAnnotation.FieldName) > 0 {
		edgeName = edgeAnnotation.FieldName
	}
	// >

	fieldNum := int32(edgeAnnotation.Number)
	fieldDesc := &descriptorpb.FieldDescriptorProto{
		Number: &fieldNum,
		Name:   &edgeName,
		Type:   &t,
	}

	if !e.Unique {
		fieldDesc.Label = &repeatedFieldLabel
	}

	relType, err := extractGenTypeByName(a.graph, msgTypeName)
	if err != nil {
		return nil, err
	}
	dstAnnotation, err := extractMessageAnnotation(relType)
	if err != nil || !dstAnnotation.Generate {
		return nil, fmt.Errorf("entproto: message %q is not generated", msgTypeName)
	}

	sourceAnnotation, err := extractMessageAnnotation(source)
	if err != nil {
		return nil, err
	}
	if sourceAnnotation.Package == dstAnnotation.Package {
		fieldDesc.TypeName = &msgTypeName
	} else {
		fqn := dstAnnotation.Package + "." + msgTypeName
		fieldDesc.TypeName = &fqn
	}

	if e.Optional {
		t := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
		fieldDesc.Type = &t
		fieldDesc.TypeName = strptr("google.protobuf.StringValue")
	}

	fieldDesc.JsonName = strptr(*fieldDesc.Name)

	return fieldDesc, nil
}

func toProtoEnumDescriptor(fld *gen.Field) (*descriptorpb.EnumDescriptorProto, error) {
	enumAnnotation, err := extractEnumAnnotation(fld)
	if err != nil {
		return nil, err
	}

	if err := enumAnnotation.Verify(fld); err != nil {
		return nil, err
	}

	enumName := pascal(fld.Name)
	dp := &descriptorpb.EnumDescriptorProto{
		Name:  strptr(enumName),
		Value: []*descriptorpb.EnumValueDescriptorProto{},
	}

	if !fld.Default {
		dp.Value = append(dp.Value, &descriptorpb.EnumValueDescriptorProto{
			Number: int32ptr(0),
			Name:   strptr(strings.ToUpper(snake(fld.Name)) + "_UNSPECIFIED"),
		})
	}

	for _, opt := range fld.Enums {
		dp.Value = append(dp.Value, &descriptorpb.EnumValueDescriptorProto{
			Number: int32ptr(enumAnnotation.Options[opt.Value]),
			Name:   strptr(strings.ToUpper(snake(opt.Value))),
		})
	}

	return dp, nil
}

func toProtoFieldDescriptor(f *gen.Field) (*descriptorpb.FieldDescriptorProto, error) {
	fieldDesc := &descriptorpb.FieldDescriptorProto{
		Name: &f.Name,
	}
	fann, err := extractFieldAnnotation(f)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// < ignore this field
	if fann == nil {
		return nil, nil
	}
	// >

	// < fieldName
	if len(fann.FieldName) > 0 {
		fieldDesc.Name = &fann.FieldName
	}
	// >

	// < field comment
	comment := extractFieldCommnt(f)
	if len(comment) > 0 {
		if fieldDesc.Options == nil {
			fieldDesc.Options = &descriptorpb.FieldOptions{}
		}
		proto.SetExtension(fieldDesc.Options, options.E_Openapiv2Field, &options.JSONSchema{Description: comment})
	}
	// >

	fieldNumber := int32(fann.Number)
	if fieldNumber == 1 && strings.ToUpper(f.Name) != "ID" {
		return nil, fmt.Errorf("entproto: field %q has number 1 which is reserved for id", f.Name)
	}
	fieldDesc.Number = &fieldNumber
	if fann.Type != descriptorpb.FieldDescriptorProto_Type(0) {
		fieldDesc.Type = &fann.Type
		if len(fann.TypeName) > 0 {
			fieldDesc.TypeName = &fann.TypeName
		}
		return fieldDesc, nil
	}
	typeDetails, err := extractProtoTypeDetails(f)
	if err != nil {
		log.Println(err, " +  ", f.Type.Type)
		return nil, err
	}
	fieldDesc.Type = &typeDetails.protoType
	if typeDetails.messageName != "" {
		fieldDesc.TypeName = &typeDetails.messageName
	}

	fieldDesc.JsonName = strptr(*fieldDesc.Name)

	return fieldDesc, nil
}

func extractProtoTypeDetails(f *gen.Field) (fieldType, error) {
	cfg, ok := typeMap[f.Type.Type]
	if !ok || cfg.unsupported {
		log.Println("unsupprted here")
		return fieldType{}, unsupportedTypeError{Type: f.Type}
	}
	if f.Optional {
		if cfg.optionalType == "" {
			return fieldType{}, unsupportedTypeError{Type: f.Type}
		}
		return fieldType{
			protoType:   descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
			messageName: cfg.optionalType,
		}, nil
	}
	name := cfg.msgTypeName
	if cfg.namer != nil {
		name = cfg.namer(f)
	}
	return fieldType{
		protoType:   cfg.pbType,
		messageName: name,
	}, nil
}

type fieldType struct {
	messageName string
	protoType   descriptorpb.FieldDescriptorProto_Type
}

func strptr(s string) *string {
	return &s
}

func int32ptr(i int32) *int32 {
	return &i
}

func extractGenTypeByName(graph *gen.Graph, name string) (*gen.Type, error) {
	for _, sch := range graph.Nodes {
		if sch.Name == name {
			return sch, nil
		}
	}
	return nil, fmt.Errorf("entproto: could not find schema %q in graph", name)
}

func dedupe(s []string) []string {
	out := make([]string, 0, len(s))
	seen := make(map[string]struct{})
	for _, item := range s {
		if _, skip := seen[item]; skip {
			continue
		}
		out = append(out, item)
		seen[item] = struct{}{}
	}
	return out
}
