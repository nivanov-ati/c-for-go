package parser

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/cznic/c/internal/cc"
)

type Config struct {
	Arch               string   `yaml:"Arch"`
	CustomDefinesPath  string   `yaml:"CustomDefinesPath"`
	WebIncludesEnabled bool     `yaml:"WebIncludesEnabled"`
	WebIncludePrefix   string   `yaml:"WebIncludePrefix"`
	IncludePaths       []string `yaml:"IncludePaths"`
	TargetPaths        []string `yaml:"TargetPaths"`
	SourceBody         string   `yaml:"SourceBody"`
	archBits           TargetArchBits
}

func NewConfig(paths ...string) *Config {
	return &Config{
		TargetPaths: paths,
	}
}

func checkConfig(cfg *Config) *Config {
	if cfg == nil {
		cfg = &Config{}
	}
	if arch, ok := arches[cfg.Arch]; !ok {
		// default to 64-bit arch
		cfg.archBits = Arch64
	} else if arch != Arch32 && arch != Arch64 {
		// default to 64-bit arch
		cfg.archBits = Arch64
	}
	return cfg
}

type Parser struct {
	cfg        *Config
	ccCfg      *cc.ParseConfig
	predefined string
}

func New(cfg *Config) (*Parser, error) {
	p := &Parser{
		cfg: checkConfig(cfg),
	}
	if len(p.cfg.TargetPaths) > 0 {
		// workaround for cznic's cc (it panics if supplied path is a dir)
		var saneFiles []string
		for _, path := range p.cfg.TargetPaths {
			if !filepath.IsAbs(path) {
				if hPath, err := findFile(path, p.cfg.IncludePaths); err != nil {
					err = fmt.Errorf("parser: file specified but not found: %s (include paths: %s)",
						path, strings.Join(p.cfg.IncludePaths, ", "))
					return nil, err
				} else {
					path = hPath
				}
			}
			if info, err := os.Stat(path); err != nil || info.IsDir() {
				continue
			}
			if absPath, err := filepath.Abs(path); err != nil {
				path = absPath
			}
			saneFiles = append(saneFiles, path)
		}
		p.cfg.TargetPaths = saneFiles
	} else {
		return nil, errors.New("parser: no target paths specified")
	}

	if def, ok := predefines[p.cfg.archBits]; !ok {
		p.predefined = predefinedBase
	} else {
		p.predefined = def
	}
	if len(p.cfg.CustomDefinesPath) > 0 {
		if buf, err := ioutil.ReadFile(p.cfg.CustomDefinesPath); err != nil {
			return nil, errors.New("parser: custom defines file provided but can't be read")
		} else if len(buf) > 0 {
			p.predefined = fmt.Sprintf("%s\n// custom defines below\n%s", p.predefined, buf)
		}
	}
	if ccCfg, err := p.ccParserConfig(); err != nil {
		return nil, err
	} else {
		p.ccCfg = ccCfg
	}
	return p, nil
}

func findFile(path string, includePaths []string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	for _, inc := range includePaths {
		result := filepath.Join(inc, path)
		if _, err := os.Stat(result); err == nil {
			return result, nil
		}
	}
	return "", errors.New("not found")
}

func (p *Parser) ccParserConfig() (*cc.ParseConfig, error) {
	ccCfg := &cc.ParseConfig{
		Predefined:         p.predefined,
		Paths:              p.cfg.TargetPaths,
		Body:               []byte(p.cfg.SourceBody),
		SysIncludePaths:    p.cfg.IncludePaths,
		WebIncludesEnabled: p.cfg.WebIncludesEnabled,
		WebIncludePrefix:   p.cfg.WebIncludePrefix,
	}
	if err := cc.CheckParseConfig(ccCfg); err != nil {
		return nil, err
	}
	return ccCfg, nil
}

func (p *Parser) Parse() (unit *cc.TranslationUnit, err error) {
	// this works as easy as this only with patched cc package, beware when using the vanilla cznic/cc.
	return cc.Parse(p.ccCfg)
}