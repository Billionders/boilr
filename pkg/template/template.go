package template

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"

	"github.com/Billionders/boilr/pkg/boilr"
	"github.com/Billionders/boilr/pkg/prompt"
	"github.com/Billionders/boilr/pkg/util/osutil"
	"github.com/Billionders/boilr/pkg/util/stringutil"
	"github.com/Billionders/boilr/pkg/util/tlog"
)

// Interface is contains the behavior of boilr templates.
type Interface interface {
	// Executes the template on the given target directory path.
	Execute(string) error

	// If used, the template will execute using default values.
	UseDefaultValues()

	// Returns the metadata of the template.
	Info() Metadata
}

func (t dirTemplate) Info() Metadata {
	return t.Metadata
}

// Get retrieves the template from a path.
func Get(path string) (Interface, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// TODO make context optional
	ctxt, err := func(fname string) (map[string]interface{}, error) {
		f, err := os.Open(fname)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}

			return nil, err
		}
		defer f.Close()

		buf, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal(buf, &metadata); err != nil {
			return nil, err
		}

		return metadata, nil
	}(filepath.Join(absPath, boilr.ContextFileName))

	metadataExists, err := osutil.FileExists(filepath.Join(absPath, boilr.TemplateMetadataName))
	if err != nil {
		return nil, err
	}

	md, err := func() (Metadata, error) {
		if !metadataExists {
			return Metadata{}, nil
		}

		b, err := os.ReadFile(filepath.Join(absPath, boilr.TemplateMetadataName))
		if err != nil {
			return Metadata{}, err
		}

		var m Metadata
		if err := json.Unmarshal(b, &m); err != nil {
			return Metadata{}, err
		}

		return m, nil
	}()

	return &dirTemplate{
		Context:  ctxt,
		FuncMap:  FuncMap,
		Path:     filepath.Join(absPath, boilr.TemplateDirName),
		Metadata: md,
	}, err
}

type dirTemplate struct {
	Path     string
	Context  map[string]interface{}
	FuncMap  template.FuncMap
	Metadata Metadata

	alignment         string
	ShouldUseDefaults bool
}

func (t *dirTemplate) UseDefaultValues() {
	t.ShouldUseDefaults = true
}

func (t *dirTemplate) BindPrompts() {
	for s, v := range t.Context {
		if m, ok := v.(map[string]interface{}); ok {
			advancedMode := prompt.New(s, false)

			for k, v2 := range m {
				if t.ShouldUseDefaults {
					t.FuncMap[k] = func() interface{} {
						switch v2 := v2.(type) {
						// First is the default value if it's a slice
						case []interface{}:
							return v2[0]
						}

						return v2
					}
				} else {
					v, p := v2, prompt.New(k, v2)

					t.FuncMap[k] = func() interface{} {
						if val := advancedMode().(bool); val {
							return p()
						}

						return v
					}
				}
			}

			continue
		}

		if t.ShouldUseDefaults {
			t.FuncMap[s] = func() interface{} {
				switch v := v.(type) {
				// First is the default value if it's a slice
				case []interface{}:
					return v[0]
				}

				return v
			}
		} else {
			t.FuncMap[s] = prompt.New(s, v)
		}
	}
}

// sanitizePathForWindows 为 Windows 清理路径中的非法字符
// Windows 不允许以下字符在文件名中: < > : " / \ | ? * { }
func sanitizePathForWindows(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}

	// Windows 禁用的字符: < > : " | ? * { }
	// 注意：/ 和 \ 是路径分隔符，需要保留
	// 冒号 : 仅在盘符处允许，但在文件名中非法
	invalidChars := "<>:\"| ?*{}"

	// 对路径的每个部分分别处理，保留路径分隔符
	parts := strings.Split(path, string(filepath.Separator))
	for i, part := range parts {
		part = strings.Map(func(r rune) rune {
			if strings.ContainsRune(invalidChars, r) {
				return '_' // 替换为下划线
			}
			return r
		}, part)
		parts[i] = part
	}

	return filepath.Join(parts...)
}

// Execute fills the template with the project metadata.
func (t *dirTemplate) Execute(dirPrefix string) error {
	t.BindPrompts()

	isOnlyWhitespace := func(buf []byte) bool {
		wsre := regexp.MustCompile(`\S`)

		return !wsre.Match(buf)
	}

	// TODO create io.ReadWriter from string
	// TODO refactor name manipulation
	return filepath.Walk(t.Path, func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Path relative to the root of the template directory
		oldName, err := filepath.Rel(t.Path, filename)
		if err != nil {
			return err
		}

		// ===== 关键修复：Windows 路径分隔符问题 =====
		// 在 Windows 上，filepath.Rel 返回使用 \ 的路径
		// 但 Go 模板引擎会将 \ 视为转义字符
		// 因此需要规范化为 Unix 风格的路径用于模板处理
		templatePath := strings.ReplaceAll(oldName, "\\", "/")

		buf := stringutil.NewString("")

		// TODO translate errors into meaningful ones
		fnameTmpl := template.Must(template.
			New("file name template").
			Option(Options...).
			Funcs(t.FuncMap).
			Parse(templatePath)) // ← 使用规范化后的路径

		if err := fnameTmpl.Execute(buf, nil); err != nil {
			return err
		}

		newName := buf.String()

		// Windows 特定处理：清理路径中的非法字符
		newName = sanitizePathForWindows(newName)

		target := filepath.Join(dirPrefix, newName)

		if info.IsDir() {
			if err := os.Mkdir(target, 0755); err != nil {
				if !os.IsExist(err) {
					return err
				}
			}
		} else {
			fi, err := os.Lstat(filename)
			if err != nil {
				return err
			}

			// Delete target file if it exists
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, fi.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			defer func(fname string) {
				contents, err := os.ReadFile(fname)
				if err != nil {
					tlog.Debug(fmt.Sprintf("couldn't read the contents of file %q, got error %q", fname, err))
					return
				}

				if isOnlyWhitespace(contents) {
					os.Remove(fname)
					return
				}
			}(f.Name())

			contentsTmpl := template.Must(template.
				New("file contents template").
				Option(Options...).
				Funcs(t.FuncMap).
				ParseFiles(filename))

			fileTemplateName := filepath.Base(filename)

			if err := contentsTmpl.ExecuteTemplate(f, fileTemplateName, nil); err != nil {
				return err
			}

			if !t.ShouldUseDefaults {
				tlog.Success(fmt.Sprintf("Created %s", newName))
			}
		}

		return nil
	})
}
