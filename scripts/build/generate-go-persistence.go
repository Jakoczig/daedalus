package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	for _, pkg := range []string{"shipstate", "paritystate", "navstate", "explorestate", "activitystate", "navcontext"} {
		dir := filepath.Join("go/internal", pkg)
		fs := token.NewFileSet()
		files, e := parser.ParseDir(fs, dir, func(i os.FileInfo) bool {
			return strings.HasSuffix(i.Name(), ".go") && !strings.HasSuffix(i.Name(), "_test.go") && i.Name() != "persistence.go"
		}, 0)
		if e != nil {
			panic(e)
		}
		p := files[pkg]
		imports := map[string]string{}
		used := map[string]bool{}
		var body bytes.Buffer
		names := []string{}
		for name := range p.Files {
			names = append(names, name)
		}
		slicesSort(names)
		for _, name := range names {
			f := p.Files[name]
			for _, i := range f.Imports {
				path, _ := strconv.Unquote(i.Path.Value)
				parts := strings.Split(path, "/")
				key := parts[len(parts)-1]
				if i.Name != nil {
					key = i.Name.Name
				}
				imports[key] = path
			}
			for _, d := range f.Decls {
				g, ok := d.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, sp := range g.Specs {
					t, ok := sp.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := t.Type.(*ast.StructType)
					if !ok {
						continue
					}
					private := false
					for _, field := range st.Fields.List {
						for _, n := range field.Names {
							if !ast.IsExported(n.Name) {
								private = true
							}
						}
					}
					if !private {
						continue
					}
					var fields, values, assign bytes.Buffer
					for _, field := range st.Fields.List {
						var typ bytes.Buffer
						format.Node(&typ, fs, field.Type)
						for _, n := range field.Names {
							if n.Name == "cached" {
								continue
							}
							ast.Inspect(field.Type, func(n ast.Node) bool {
								if s, ok := n.(*ast.SelectorExpr); ok {
									if id, ok := s.X.(*ast.Ident); ok {
										used[id.Name] = true
									}
								}
								return true
							})
							x := "F" + strings.ToUpper(n.Name[:1]) + n.Name[1:]
							fmt.Fprintf(&fields, "%s %s;", x, typ.String())
							fmt.Fprintf(&values, "%s:v.%s,", x, n.Name)
							fmt.Fprintf(&assign, "v.%s=p.%s;", n.Name, x)
						}
					}
					fmt.Fprintf(&body, "type persisted%s struct{%s}\nfunc(v %s)GobEncode()([]byte,error){var b bytes.Buffer;err:=gob.NewEncoder(&b).Encode(persisted%s{%s});return b.Bytes(),err}\nfunc(v *%s)GobDecode(raw []byte)error{var p persisted%s;if err:=gob.NewDecoder(bytes.NewReader(raw)).Decode(&p);err!=nil{return err};%s;return nil}\n", t.Name, fields.String(), t.Name, t.Name, values.String(), t.Name, t.Name, assign.String())
				}
			}
		}
		var out bytes.Buffer
		fmt.Fprintf(&out, "// Code generated for private durable reducer mementos; not a transport DTO.\n// Regenerate deliberately when reducer state changes; CacheFormat invalidates old caches.\npackage %s\nimport(\"bytes\";\"encoding/gob\";", pkg)
		for key := range used {
			if path := imports[key]; path != "" {
				fmt.Fprintf(&out, "%s %q;", key, path)
			}
		}
		fmt.Fprintf(&out, ")\nconst CacheFormat=%q\n%s", fmt.Sprintf("%x", sha256.Sum256(body.Bytes())), body.String())
		raw, e := format.Source(out.Bytes())
		if e != nil {
			panic(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "persistence.go"), raw, 0600); e != nil {
			panic(e)
		}
	}
}
func slicesSort(v []string) {
	for i := range v {
		for j := i + 1; j < len(v); j++ {
			if v[j] < v[i] {
				v[i], v[j] = v[j], v[i]
			}
		}
	}
}
