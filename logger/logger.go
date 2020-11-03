package logger

import (
	"database/sql/driver"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"time"
	"unicode"

	"github.com/ybkuroki/go-webapp-sample/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v2"
)

var logger *Logger

// Config is
type Config struct {
	ZapConfig zap.Config        `json:"zap_config" yaml:"zap_config"`
	LogRotate lumberjack.Logger `json:"log_rotate" yaml:"log_rotate"`
}

// Logger is an alternative implementation of *gorm.Logger
type Logger struct {
	zap *zap.SugaredLogger
}

// GetLogger is return Logger
func GetLogger() *Logger {
	return logger
}

// SetLogger sets logger
func SetLogger(log *Logger) {
	logger = log
}

// GetZapLogger returns zapSugaredLogger
func GetZapLogger() *zap.SugaredLogger {
	return logger.zap
}

// NewLogger create logger object for *gorm.DB from *echo.Logger
func NewLogger(zap *zap.SugaredLogger) *Logger {
	return &Logger{zap: zap}
}

// InitLogger initialize logger.
func InitLogger() {
	configYaml, err := ioutil.ReadFile("./zaplogger." + *config.GetEnv() + ".yml")
	if err != nil {
		fmt.Printf("Failed to read zap logger configuration: %s", err)
	}
	var myConfig *Config
	if err := yaml.Unmarshal(configYaml, &myConfig); err != nil {
		fmt.Printf("Failed to read zap logger configuration: %s", err)
	}
	zap, err := build(myConfig)
	if err != nil {
		fmt.Printf("Error")
	}
	sugar := zap.Sugar()
	// set package varriable logger.
	logger = NewLogger(sugar)
	logger.zap.Infof("Success to read zap logger configuration: zaplogger." + *config.GetEnv() + ".yml")
	_ = zap.Sync()
}

func build(cfg *Config) (*zap.Logger, error) {
	var zapCfg zap.Config = cfg.ZapConfig
	enc, _ := newEncoder(zapCfg)
	writer, errWriter := openWriters(cfg)
	log := zap.New(zapcore.NewCore(enc, writer, zapCfg.Level), buildOptions(zapCfg, errWriter)...)
	return log, nil
}

func newEncoder(cfg zap.Config) (zapcore.Encoder, error) {
	switch cfg.Encoding {
	case "console":
		return zapcore.NewConsoleEncoder(cfg.EncoderConfig), nil
	case "json":
		return zapcore.NewJSONEncoder(cfg.EncoderConfig), nil
	}
	return nil, fmt.Errorf("Failed to set encoder")
}

func openWriters(cfg *Config) (zapcore.WriteSyncer, zapcore.WriteSyncer) {
	writer := open(cfg.ZapConfig.OutputPaths, &cfg.LogRotate)
	errWriter := open(cfg.ZapConfig.ErrorOutputPaths, &cfg.LogRotate)
	return writer, errWriter
}

func open(paths []string, rotateCfg *lumberjack.Logger) zapcore.WriteSyncer {
	writers := make([]zapcore.WriteSyncer, 0, len(paths))
	for _, path := range paths {
		writer := newWriter(path, rotateCfg)
		writers = append(writers, writer)
	}
	writer := zap.CombineWriteSyncers(writers...)
	return writer
}

func newWriter(path string, rotateCfg *lumberjack.Logger) zapcore.WriteSyncer {
	switch path {
	case "stdout":
		return os.Stdout
	case "stderr":
		return os.Stderr
	}
	sink := zapcore.AddSync(
		&lumberjack.Logger{
			Filename:   rotateCfg.Filename,
			MaxSize:    rotateCfg.MaxSize,
			MaxBackups: rotateCfg.MaxBackups,
			MaxAge:     rotateCfg.MaxAge,
		},
	)
	return sink
}

func buildOptions(cfg zap.Config, errWriter zapcore.WriteSyncer) []zap.Option {
	opts := []zap.Option{zap.ErrorOutput(errWriter)}
	if cfg.Development {
		opts = append(opts, zap.Development())
	}

	if !cfg.DisableCaller {
		opts = append(opts, zap.AddCaller())
	}

	stackLevel := zap.ErrorLevel
	if cfg.Development {
		stackLevel = zap.WarnLevel
	}
	if !cfg.DisableStacktrace {
		opts = append(opts, zap.AddStacktrace(stackLevel))
	}
	return opts
}

// ==============================================================
// Customize SQL Logger for gorm library
// ref: https://github.com/wantedly/gorm-zap
// ref: https://github.com/jinzhu/gorm/blob/master/logger.go
// ===============================================================

// Print passes arguments to Println
func (l *Logger) Print(values ...interface{}) {
	l.Println(values)
}

// Println format & print log
func (l *Logger) Println(values []interface{}) {
	sql := createLog(values)
	if sql != "" {
		l.zap.Debugf(sql)
	}
}

// createLog returns log for output
func createLog(values []interface{}) string {
	ret := ""

	if len(values) > 1 {
		var level = values[0]

		if level == "sql" {
			ret = "[gorm] : " + createSQL(values[3].(string), getFormattedValues(values))
		}
	}

	return ret
}

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// getFormattedValues returns values of a SQL statement.
func getFormattedValues(values []interface{}) []string {
	var formattedValues []string
	for _, value := range values[4].([]interface{}) {
		indirectValue := reflect.Indirect(reflect.ValueOf(value))
		if indirectValue.IsValid() {
			value = indirectValue.Interface()
			if t, ok := value.(time.Time); ok {
				if t.IsZero() {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", "0000-00-00 00:00:00"))
				} else {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", t.Format("2006-01-02 15:04:05")))
				}
			} else if b, ok := value.([]byte); ok {
				if str := string(b); isPrintable(str) {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", str))
				} else {
					formattedValues = append(formattedValues, "'<binary>'")
				}
			} else if r, ok := value.(driver.Valuer); ok {
				if value, err := r.Value(); err == nil && value != nil {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
				} else {
					formattedValues = append(formattedValues, "NULL")
				}
			} else {
				switch value.(type) {
				case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
					formattedValues = append(formattedValues, fmt.Sprintf("%v", value))
				default:
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
				}
			}
		} else {
			formattedValues = append(formattedValues, "NULL")
		}
	}
	return formattedValues
}

// createSQL returns complete SQL with values bound to a SQL statement.
func createSQL(sql string, values []string) string {
	var (
		sqlRegexp                = regexp.MustCompile(`\?`)
		numericPlaceHolderRegexp = regexp.MustCompile(`\$\d+`)
		result                   = ""
	)
	// differentiate between $n placeholders or else treat like ?
	if numericPlaceHolderRegexp.MatchString(sql) {
		for index, value := range values {
			placeholder := fmt.Sprintf(`\$%d([^\d]|$)`, index+1)
			result = regexp.MustCompile(placeholder).ReplaceAllString(sql, value+"$1")
		}
	} else {
		formattedValuesLength := len(values)
		for index, value := range sqlRegexp.Split(sql, -1) {
			result += value
			if index < formattedValuesLength {
				result += values[index]
			}
		}
	}
	return result
}
