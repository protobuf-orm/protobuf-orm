package ormpb

func RegByNumber(n int) *Ref {
	return Ref_builder{Number: int32(n)}.Build()
}

func RefByName(name string) *Ref {
	return Ref_builder{Name: name}.Build()
}
