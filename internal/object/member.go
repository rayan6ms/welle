package object

// MemberGetter allows objects to expose fields via member access (obj.field).
type MemberGetter interface {
	GetMember(name string) (Object, bool)
}

// MemberSetter allows objects to handle member assignment (obj.field = value).
type MemberSetter interface {
	SetMember(name string, value Object) error
}
