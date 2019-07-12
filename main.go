package main

import (
	"bufio"
	"container/list"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	IncludeDef = "#include"
	StartChar  = '#'
	quotation  = '"'
	pquotation = '"'
	bracket    = '<'
	pbracket   = '>'
	YcmUrl     = "https://raw.githubusercontent.com/iohub/ycmheader/master/ycm_extra_conf.py"
)

var exts = [...]string{".cpp", ".hpp", ".h", ".cxx", ".c", ".cc"}
var verbose = false

type PathFormator func(string, string) string

func isCpp(path string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func findCpp(dir string) *list.List {
	vec := list.New()
	walker := func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() && isCpp(path) {
			if verbose {
				fmt.Println(path)
			}
			vec.PushBack(path)
		}
		return nil
	}

	filepath.Walk(dir, walker)
	return vec
}

func extractor(line string) (string, bool) {
	Lline := len(line)
	Linclude := len(IncludeDef)
	if Lline < Linclude+2 {
		return "", false
	}
	var next rune
	offset := len(IncludeDef)
	switch line[offset] {
	case bracket:
		next = pbracket
	case quotation:
		next = pquotation
	default:
		return "", false

	}
	for i := Linclude + 1; i < Lline; i++ {
		if line[i] == byte(next) {
			return line[Linclude+1 : i], true
		}
	}

	return "", true
}

func loadConf(fname string) (string, error) {
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		cmd := exec.Command("wget", YcmUrl, "-O", fname)
		fmt.Printf("wget %v -O %v\n", YcmUrl, fname)
		cmd.Start()
	}
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return "", err
	}
	str := string(b)
	return str, nil
}

func saveConf(fname string, conf string) error {
	return ioutil.WriteFile(fname, []byte(conf), 0644)
}

func includeOf(fname string, hmap map[string]int) error {
	fobj, err := os.Open(fname)
	if err != nil {
		return err
	}

	defer fobj.Close()
	reader := bufio.NewReader(fobj)
	var line string
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		if len(line) == 0 || line[0] != StartChar {
			continue
		}
		line = strings.Replace(line, " ", "", -1)
		if !strings.HasPrefix(line, IncludeDef) {
			continue
		}
		header, ok := extractor(line)
		if !ok {
			continue
		}
		if verbose {
			fmt.Printf("-- %s\n", header)
		}
		hmap[header] += 1
	}

	return nil
}

func PathFormatorV1(formated, path string) string {
	return formated + fmt.Sprintf("'-I%v',\n", path)
}

func PathFormatorV2(formated, path string) string {
	return formated + fmt.Sprintf("'-I',\n'%v',\n", path)
}

func GenIncludeArg(includeSet map[string]int, fn PathFormator) string {
	paths := ""
	for p, _ := range includeSet {
		paths = fn(paths, p)
	}
	return paths
}

func main() {
	var (
		projDir string
		exclude string
		format  string
	)

	flag.StringVar(&projDir, "path", ".", "absolute path of c/c++ project")
	flag.BoolVar(&verbose, "v", false, "true for verbose mode")
	flag.StringVar(&exclude, "ex", "third_party", "exclude path")
	flag.StringVar(&format, "format", "v2", "format version: v1, v2")
	flag.Parse()

	vec := findCpp(projDir)
	hmap := make(map[string]int)
	for f := vec.Front(); f != nil; f = f.Next() {
		fpath := f.Value.(string)
		if strings.Contains(fpath, exclude) {
			continue
		}

		if verbose {
			fmt.Printf("[%s]\n", fpath)
		}
		includeOf(fpath, hmap)
	}
	found := map[string]int{
		".": 1,
	}
	count := 0
	for header, _ := range hmap {
		// fmt.Printf("[%s]\n", header)
		count += 1
		ok := false
		for f := vec.Front(); f != nil; f = f.Next() {
			path := f.Value.(string)
			if idx := strings.Index(path, header); idx != -1 && idx-1 > 0 && path[idx-1] == '/' {
				ok = true
				found[path[0:idx]] += 1
				break
			}
		}
		if !ok && verbose {
			fmt.Printf("[system]: %s\n", header)
		}
	}
	fmt.Printf("Total header: %d\n", count)
	fn := PathFormatorV1
	if format == "v2" {
		fn = PathFormatorV2
	}
	paths := GenIncludeArg(found, fn)
	fmt.Println("Gen Include:\n", paths)
	conf := "/tmp/ycm_extra_conf.py"
	if str, err := loadConf(conf); err == nil {
		nconf := strings.Replace(str, "$IncludePaths", paths, 1)
		saveConf(".ycm_extra_conf.py", nconf)
	}
}
