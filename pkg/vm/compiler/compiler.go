package compiler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/types"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/vm"
	"golang.org/x/tools/go/loader"
)

const fileExt = "avm"

// Options contains all the parameters that affect the behaviour of the compiler.
type Options struct {
	// The extension of the output file default set to .avm
	Ext string

	// The name of the output file.
	Outfile string

	// Debug outputs a hex encoded string of the generated bytecode.
	Debug bool
}

type buildInfo struct {
	initialPackage string
	program        *loader.Program
}

// Compile compiles a Go program into bytecode that can run on the NEO virtual machine.
func Compile(r io.Reader, o *Options) ([]byte, error) {
	conf := loader.Config{ParserMode: parser.ParseComments}
	f, err := conf.ParseFile("", r)
	if err != nil {
		return nil, err
	}
	conf.CreateFromFiles("", f)

	prog, err := conf.Load()
	if err != nil {
		return nil, err
	}

	ctx := &buildInfo{
		initialPackage: f.Name.Name,
		program:        prog,
	}

	buf, err := CodeGen(ctx)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type archive struct {
	f        *ast.File
	typeInfo *types.Info
}

// CompileAndSave will compile and save the file to disk.
func CompileAndSave(src string, o *Options) error {
	if !strings.HasSuffix(src, ".go") {
		return fmt.Errorf("%s is not a Go file", src)
	}
	o.Outfile = strings.TrimSuffix(o.Outfile, fmt.Sprintf(".%s", fileExt))
	if len(o.Outfile) == 0 {
		o.Outfile = strings.TrimSuffix(src, ".go")
	}
	if len(o.Ext) == 0 {
		o.Ext = fileExt
	}
	b, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	b, err = Compile(bytes.NewReader(b), o)
	if err != nil {
		return fmt.Errorf("error while trying to compile smart contract file: %v", err)
	}

	log.Println(hex.EncodeToString(b))

	out := fmt.Sprintf("%s.%s", o.Outfile, o.Ext)
	return ioutil.WriteFile(out, b, os.ModePerm)
}

// CompileAndInspect compiles the program and dumps the opcode in a user friendly format.
func CompileAndInspect(src string) error {
	b, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	b, err = Compile(bytes.NewReader(b), &Options{})
	if err != nil {
		return err
	}

	v := vm.New()
	v.LoadScript(b)
	v.PrintOps()
	return nil
}

func gopath() string {
	gopath := os.Getenv("GOPATH")
	if len(gopath) == 0 {
		gopath = build.Default.GOPATH
	}
	return gopath
}

func init() {
	log.SetFlags(0)
}
