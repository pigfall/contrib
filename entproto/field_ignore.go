package entproto

import(
	"entgo.io/ent/schema"
)

const FieldIgnoreAnnotation="ProtoFieldIgnore"


func FieldIgnore() schema.Annotation{
	return &pbFieldIgnore{}
}

type pbFieldIgnore struct{}

func (pbFieldIgnore) Name() string{
	return FieldIgnoreAnnotation
}
