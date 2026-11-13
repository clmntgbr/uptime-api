package importer

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/library"
)

// curlShortWithValue lists the short option letters that take a value.
const curlShortWithValue = "XHdFobcuAemxDEKPQTUwYzCr"

// maxCurlSeconds is the largest --max-time value that fits a time.Duration.
const maxCurlSeconds = float64(math.MaxInt64 / int64(time.Second))

// curlLongAliases maps short options to their long names.
var curlLongAliases = map[string]string{
	"-X": "--request", "-H": "--header", "-d": "--data", "-F": "--form",
	"-o": "--output", "-b": "--cookie", "-c": "--cookie-jar", "-u": "--user",
	"-A": "--user-agent", "-e": "--referer", "-m": "--max-time", "-x": "--proxy",
	"-k": "--insecure", "-L": "--location", "-G": "--get", "-I": "--head",
	"-4": "--ipv4", "-6": "--ipv6", "-s": "--silent", "-S": "--show-error",
	"-v": "--verbose", "-i": "--include", "-#": "--progress-bar",
}

// curlLongWithValue lists long options that take a value when not written as --opt=value.
var curlLongWithValue = map[string]bool{
	"--request": true, "--header": true, "--data": true, "--data-raw": true,
	"--data-binary": true, "--data-ascii": true, "--data-urlencode": true,
	"--url": true, "--user": true, "--user-agent": true, "--referer": true,
	"--max-time": true, "--connect-timeout": true, "--proxy": true, "--form": true,
	"--output": true, "--cookie": true, "--cookie-jar": true, "--write-out": true,
	"--upload-file": true, "--dump-header": true, "--cert": true, "--retry": true,
}

// curlHeader is one header in command-line order; later entries win.
type curlHeader struct {
	name, value string
}

// curlRequest collects what one curl command asks for.
type curlRequest struct {
	url       string
	method    string // explicit -X, uppercased
	headers   []curlHeader
	dataParts []string
	hasData   bool
	getMode   bool
	headMode  bool
	timeout   time.Duration
	insecure  bool
	proxy     string
	ipVersion domaincall.IPVersion
	ws        warningList
}

// parseCurl converts one or more curl commands into candidates.
func parseCurl(content []byte) (library.ImportResult, error) {
	var res library.ImportResult
	for i, words := range splitShellCommands(string(content)) {
		if words[0] != "curl" {
			addSkipped(&res, firstWords(words, 3), "not a curl command")
			continue
		}
		cmd := parseCurlArgs(words[1:])
		if !isCurlURL(cmd.url) {
			addSkipped(&res, fmt.Sprintf("command %d", i+1), "no URL")
			continue
		}
		addCandidate(&res, "cmd", cmd.definition(), cmd.ws)
	}
	if len(res.Candidates) == 0 && len(res.Skipped) == 0 {
		return res, importError("no curl command found")
	}
	return res, nil
}

// isCurlURL accepts absolute URLs and templated ones such as {{base_url}}/x.
func isCurlURL(raw string) bool {
	return strings.HasPrefix(raw, "{{") || strings.Contains(raw, "://")
}

// parseCurlArgs reads the arguments that follow "curl".
func parseCurlArgs(args []string) *curlRequest {
	c := &curlRequest{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			for _, rest := range args[i+1:] {
				c.positional(rest)
			}
			return c
		case strings.HasPrefix(arg, "--"):
			name, value, hasValue := strings.Cut(arg, "=")
			if !curlLongWithValue[name] {
				c.flag(name)
				continue
			}
			if !hasValue {
				value, i = takeNext(args, i)
			}
			c.option(name, value)
		case len(arg) > 1 && arg[0] == '-':
			for j := 1; j < len(arg); j++ {
				letter := arg[j]
				name := "-" + string(letter)
				if strings.IndexByte(curlShortWithValue, letter) < 0 {
					c.flag(name)
					continue
				}
				value := arg[j+1:]
				if value == "" {
					value, i = takeNext(args, i)
				}
				c.option(name, value)
				break
			}
		default:
			c.positional(arg)
		}
	}
	return c
}

// takeNext returns the argument after index i and its index, or "" and i when
// there is none.
func takeNext(args []string, i int) (string, int) {
	if i+1 < len(args) {
		return args[i+1], i + 1
	}
	return "", i
}

func curlName(name string) string {
	if long, ok := curlLongAliases[name]; ok {
		return long
	}
	return name
}

func (c *curlRequest) positional(value string) {
	if c.url == "" {
		c.url = value
	}
}

// option handles an option that takes a value. name may be short or long.
func (c *curlRequest) option(name, value string) {
	switch name = curlName(name); name {
	case "--request":
		c.method = strings.ToUpper(value)
	case "--header":
		c.addHeader(value)
	case "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode":
		c.addData(name, value)
	case "--form":
		c.hasData = true
		c.ws.add(warnUnsupportedOption, "--form", "Multipart form data is not supported; the option was ignored")
	case "--url":
		c.url = value
	case "--user":
		c.headers = append(c.headers, curlHeader{name: "Authorization", value: basicAuth(value)})
	case "--user-agent":
		c.headers = append(c.headers, curlHeader{name: "User-Agent", value: value})
	case "--referer":
		c.headers = append(c.headers, curlHeader{name: "Referer", value: value})
	case "--max-time":
		c.setMaxTime(value)
	case "--connect-timeout":
		// Not mapped: the monitor applies its own timeout.
	case "--proxy":
		c.proxy = value
	default:
		c.ws.add(warnUnsupportedOption, name, "Option %s is not supported and was ignored", name)
	}
}

// flag handles an option that takes no value.
func (c *curlRequest) flag(name string) {
	switch name = curlName(name); name {
	case "--insecure":
		c.insecure = true
	case "--get":
		c.getMode = true
	case "--head":
		c.headMode = true
	case "--ipv4":
		c.ipVersion = domaincall.IPv4
	case "--ipv6":
		c.ipVersion = domaincall.IPv6
	case "--location":
		c.ws.add(warnUnsupportedOption, "--location", "Redirects are not followed by the monitor; the option was ignored")
	case "--silent", "--show-error", "--verbose", "--include", "--compressed", "--progress-bar":
		// Output options without meaning for the monitor.
	default:
		c.ws.add(warnUnsupportedOption, name, "Option %s is not supported and was ignored", name)
	}
}

// addHeader parses "Name: value". "Name;" (curl's empty-header syntax) is skipped silently.
func (c *curlRequest) addHeader(raw string) {
	name, value, found := strings.Cut(raw, ":")
	if !found {
		if !strings.HasSuffix(strings.TrimSpace(raw), ";") {
			c.ws.add(warnUnsupportedOption, "--header", "Header %q is not in Name: value form and was ignored", raw)
		}
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		c.ws.add(warnUnsupportedOption, "--header", "Header %q has no name and was ignored", raw)
		return
	}
	c.headers = append(c.headers, curlHeader{name: name, value: strings.TrimSpace(value)})
}

// addData records a data option. @file references are not supported.
func (c *curlRequest) addData(name, value string) {
	c.hasData = true
	if name != "--data-raw" && strings.HasPrefix(value, "@") {
		c.ws.add(warnUnsupportedOption, name, "File references (@file) are not supported; the body was not imported")
		return
	}
	c.dataParts = append(c.dataParts, value)
}

func (c *curlRequest) setMaxTime(value string) {
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || !(seconds >= 0 && seconds <= maxCurlSeconds) {
		c.ws.add(warnUnsupportedOption, "--max-time", "Invalid max-time %q was ignored", value)
		return
	}
	c.timeout = time.Duration(seconds * float64(time.Second))
}

// basicAuth encodes -u credentials. Without a colon, curl prompts for the
// password; the empty password is used instead.
func basicAuth(credentials string) string {
	if !strings.Contains(credentials, ":") {
		credentials += ":"
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
}

// methodName applies curl's defaults: -X, then -G and -I, then POST when data is sent.
func (c *curlRequest) methodName() string {
	switch {
	case c.method != "":
		return c.method
	case c.getMode:
		return "GET"
	case c.headMode:
		return "HEAD"
	case c.hasData:
		return "POST"
	default:
		return "GET"
	}
}

func (c *curlRequest) definition() domaincall.Definition {
	def := domaincall.Definition{
		Method:    c.methodName(),
		URL:       c.url,
		Headers:   map[string]string{},
		Query:     map[string]string{},
		Timeout:   c.timeout,
		IPVersion: c.ipVersion,
		ProxyURL:  c.proxy,
	}
	for _, h := range c.headers {
		setHeader(def.Headers, h.name, h.value)
	}
	joined := strings.Join(c.dataParts, "&")
	if c.getMode {
		def.Query = parseQueryPairs(joined)
	} else {
		def.Body = joined
	}
	if c.insecure {
		verifyTLS := false
		def.VerifyTLS = &verifyTLS
	}
	def.Name = endpointName(def.Method, c.url)
	return def
}

func firstWords(words []string, n int) string {
	return strings.Join(words[:min(n, len(words))], " ")
}

// splitShellCommands splits pasted shell text into commands, and each command
// into words. An unquoted newline ends a command; backslash-newline continues it.
// Quotes, backslash escapes and ANSI-C $'...' quoting are interpreted.
func splitShellCommands(src string) [][]string {
	var (
		commands [][]string
		words    []string
		word     strings.Builder
		inWord   bool
	)
	endWord := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	endCommand := func() {
		endWord()
		if len(words) > 0 {
			commands = append(commands, words)
			words = nil
		}
	}
	for i := 0; i < len(src); i++ {
		ch := src[i]
		switch ch {
		case ' ', '\t', '\r':
			endWord()
		case '\n':
			endCommand()
		case '\\':
			if i+1 >= len(src) {
				continue
			}
			next := src[i+1]
			if next == '\n' {
				i++
				continue
			}
			if next == '\r' && i+2 < len(src) && src[i+2] == '\n' {
				i += 2
				continue
			}
			word.WriteByte(next)
			inWord = true
			i++
		case '\'':
			inWord = true
			end := strings.IndexByte(src[i+1:], '\'')
			if end < 0 {
				word.WriteString(src[i+1:])
				i = len(src)
				continue
			}
			word.WriteString(src[i+1 : i+1+end])
			i += end + 1
		case '"':
			inWord = true
			i = readDoubleQuoted(src, i+1, &word)
		case '$':
			inWord = true
			if i+1 < len(src) && src[i+1] == '\'' {
				i = readANSIQuoted(src, i+2, &word)
			} else {
				word.WriteByte(ch)
			}
		default:
			inWord = true
			word.WriteByte(ch)
		}
	}
	endCommand()
	return commands
}

// readDoubleQuoted copies a double-quoted string starting at start and returns
// the index of the closing quote (or len(src) when unterminated).
func readDoubleQuoted(src string, start int, buf *strings.Builder) int {
	for j := start; j < len(src); j++ {
		ch := src[j]
		if ch == '"' {
			return j
		}
		if ch == '\\' && j+1 < len(src) {
			next := src[j+1]
			switch next {
			case '"', '\\', '$', '`':
				buf.WriteByte(next)
				j++
			case '\n':
				j++
			default:
				buf.WriteByte(ch)
			}
			continue
		}
		buf.WriteByte(ch)
	}
	return len(src)
}

// readANSIQuoted decodes a $'...' string starting at start and returns the
// index of the closing quote (or len(src) when unterminated).
func readANSIQuoted(src string, start int, buf *strings.Builder) int {
	for j := start; j < len(src); j++ {
		ch := src[j]
		if ch == '\'' {
			return j
		}
		if ch != '\\' || j+1 >= len(src) {
			buf.WriteByte(ch)
			continue
		}
		j++
		next := src[j]
		switch next {
		case 'n':
			buf.WriteByte('\n')
		case 't':
			buf.WriteByte('\t')
		case 'r':
			buf.WriteByte('\r')
		case '\'', '\\', '"':
			buf.WriteByte(next)
		case 'x':
			value, digits := 0, 0
			for digits < 2 && j+1 < len(src) {
				digit, ok := hexValue(src[j+1])
				if !ok {
					break
				}
				value = value*16 + digit
				j++
				digits++
			}
			if digits == 0 {
				buf.WriteString(`\x`)
			} else {
				buf.WriteByte(byte(value))
			}
		default:
			buf.WriteByte('\\')
			buf.WriteByte(next)
		}
	}
	return len(src)
}

func hexValue(ch byte) (int, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0'), true
	case ch >= 'a' && ch <= 'f':
		return int(ch-'a') + 10, true
	case ch >= 'A' && ch <= 'F':
		return int(ch-'A') + 10, true
	}
	return 0, false
}
