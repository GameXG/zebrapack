package parse

import (
	"fmt"

	"github.com/glycerine/zebrapack/gen"
	"github.com/glycerine/zebrapack/zebra"
)

func TranslateToZebraSchema(fs *FileSet) (*zebra.Schema, error) {

	structs := []zebra.Struct{}

	for k, ele := range fs.Identities {
		fmt.Printf("\n on k = %v\n", k)
		switch x := ele.(type) {
		case *gen.Struct:
			tr := zebra.Struct{
				StructName: x.TypeName(),
			}
			for _, f := range x.Fields {
				fld := zebra.Field{
					Zid:       f.ZebraId,
					OmitEmpty: f.OmitEmpty,
					Skip:      f.Skip,
					FieldName: f.FieldTag,
				}
				if !fld.Skip {
					fld.TypStr = f.FieldElem.TypeName()
				}
				fmt.Printf("\n in %v,  on field %#v ... fld='%#v'\n", tr.StructName, f, fld)
				tr.Fld = append(tr.Fld, fld)
			}
			structs = append(structs, tr)
			/*
				case *Ptr:
				case *Array:
				case *Slice:
				case *Map:
				case *BaseElem:
			*/
		default:
			return nil, fmt.Errorf("unhandled type %T", x)
		}
	}
	sch := &zebra.Schema{
		Structs: structs,
	}
	fmt.Printf("total number of structs: %v\n", len(structs))
	fmt.Printf("total number of fields in first struct: %v\n", len(structs[0].Fld))
	return sch, nil

}