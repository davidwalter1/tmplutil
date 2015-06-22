package tmplutil

import (
    "fmt"
    "io/ioutil"
    "strings"
    "flag"
    "html/template"
    "github.com/russross/blackfriday"
    "bytes"
    "bufio"
    "net/http"
    "regexp"
    "io"
    "log"
	"os"
	wiki     "github.com/m4tty/cajun"
)

var debug = false

var (
    Trace   *log.Logger
    Info    *log.Logger
    Warning *log.Logger
    Error   *log.Logger
)

func Init(
    traceHandle io.Writer,
    infoHandle io.Writer,
    warningHandle io.Writer,
    errorHandle io.Writer) {

    Trace = log.New(traceHandle,
        "TRACE: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Info = log.New(infoHandle,
        "INFO: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Warning = log.New(warningHandle,
        "WARNING: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Error = log.New(errorHandle,
        "ERROR: ",
        log.Ldate|log.Ltime|log.Lshortfile)
}

func LogString( r* http.Request ) string {
    return fmt.Sprintf("\"%s %s %s\" \"%s\" \"%s\"",
        r.Method,
        r.URL.String(),
        r.Proto,
        r.Referer(),
        r.UserAgent() )
}

func init() {
    flag.Parse()
	Init(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)
}

// valid path are names to process as markdown markup with template
// var WikiPath    = regexp.MustCompile("^/(wiki)/([-\\._:a-zA-Z0-9]+\\.wiki)$")
// var MdPath      = regexp.MustCompile("^/(slides|menu1|menu2|menu3|plain|test)/([-\\._:a-zA-Z0-9]+\\.md)$")
// var WikiPathRegex    		= regexp.MustCompile("^/(wiki)/(.+\\.wiki)(\\?.*)?$")
// var MdPathRegex      		= regexp.MustCompile("^/(slides|menu1|menu2|menu3|plain|test)/(.+\\.md)(\\?.*)?$")
var FilePathRegex    		= regexp.MustCompile("^(/(slides|menu1|menu2|menu3|plain|test|wiki)/)?(.+\\.md|.+\\.wiki|.+\\.html?)(\\?.*)*$")
var DirectoryListingTemplateFile = "dir.html"
var DirectoryListingTemplateText = Load( DirectoryListingTemplateFile )

var titleSplit              = "title:"
var subtitleSplit           = "subtitle:"
var classSplit              = "class:"
var noteSplit               = "note:"
var buildSplit              = "!build_lists:"
var imageSplit              = "image:"   
var backgroundSplit         = "background:"   
var h1SplitStar             = "* "   
var h2SplitStar             = "** "   
var h1SplitHash             = "# "   
var h2SplitHash             = "## "   

var templateHead            = flag.String( "templateHead",      "head.html",            	"header filename")
var templateBody            = flag.String( "templateBody",      "body.html",            	"golang formatted template filename")
var templateFoot            = flag.String( "templateFoot",      "foot.html",            	"footer filename")
var Filename                = flag.String( "filename",          "",                      	"markdown filename")
var RawFlag                 = flag.Bool  ( "raw",               false,                   	"markup a markdown filename -filename write to stdout with defaults and exit.")
var Slides                  = flag.Bool  ( "slides",            false,                   	"markup a markdown filename -filename write to stdout with slide format and exit.")
var Passthrough             = flag.Bool  ( "passthrough",       false,                   	"markup a markdown filename -filename write to stdout and exit.")
var image                   = flag.String( "image",             "../images/sphere.png",     "image with path relative to web root")
var background              = flag.String( "background-image",  "../images/background.png", "background-image with path relative to web root")
var segueText               = "segue"

type Page struct {
    Filename         string
    Title            string
    Subtitle         string
    Class            string
    Html             template.HTML
    Image            string
    BackgroundImage  string
    Segue            bool
    Note             template.HTML
}

func Log( r* http.Request ){
    fmt.Printf("\"%s %s %s\" \"%s\" \"%s\"\n",
        r.Method,
        r.URL.String(),
        r.Proto,
        r.Referer(),
        r.UserAgent())
}

type Pages []*Page


type Arglist struct {
    Title         *string
    TemplateName  *string
    WebRoot       *string
    Pages         *Pages
	Address       string
	ReadTimeout   int64
	WriteTimeout  int64
}

func parse( filename, body string ) ( *Page ) {
    text                                 := make([]string, 1)
    titleText                            := ""
    subtitleText                         := ""
    classText                            := ""
    noteText                             := ""
    imageText                            := ""
    backgroundimageText                  := *background
    buildLists                           := false
    segueSet                             := false
    for _, line := range strings.Split( body, "\n" ) {
        found := false
        tmp := ""
        // Not testing for which mode, title:/subtitle: or asterisk
        // title/subtitle mapping
        // markdown default overrides title:/subtitle:
        Metadata( line, titleSplit,      &titleText           , &found )
        Metadata( line, subtitleSplit,   &subtitleText        , &found )
        Metadata( line, h1SplitHash,     &titleText           , &found )
        Metadata( line, h2SplitHash,     &subtitleText        , &found )
        Metadata( line, h1SplitStar,     &titleText           , &found )
        Metadata( line, h2SplitStar,     &subtitleText        , &found )
        Metadata( line, classSplit,      &classText           , &found )
        Metadata( line, noteSplit,       &tmp                 , &found )
        Metadata( line, imageSplit,      &imageText           , &found )
        Metadata( line, backgroundSplit, &backgroundimageText , &found )
        IsSet   ( line, buildSplit,      &buildLists          , &found )

        if len( tmp ) > 0 {
            noteText += "\n" + tmp
        }
        if Segue( line ) {
            segueSet = true 
        }
        if ! found  {
            text = append( text, line )
        }
    }

    if len( noteText ) > 0 {
        noteText = string( blackfriday.MarkdownCommon( []byte( noteText ) ) )
    }

    html := blackfriday.MarkdownCommon( []byte( strings.Join( text, "\n" ) ) )

    if buildLists {
        text     := string( html )
        text      = strings.Replace( text, "<ul>", "<ul class=\"build\">", -1 )
        text      = strings.Replace( text, "<ol>", "<ol class=\"build\">", -1 )
        html      = []byte( text )
    }
    return &Page{ filename, titleText, subtitleText, classText, template.HTML( html ), *image, backgroundimageText, segueSet, template.HTML( noteText ) }
}

func Raw( filename, body string ) ( string ) {
    page := parse( filename, body )
    note := string( page.Note )
    if len( note ) > 0 {
        note = "\nNote\n" + note 
        return string( page.Html ) + note
    }
    return page.Title + "\n" + page.Subtitle + "\n" + string( page.Html )
}

func Parse( filename, body string ) ( *Page ) {
    page := parse( filename, body )
    return page
}

func Load( filename string ) ( *string ) {
    text, err := ioutil.ReadFile( filename )
    if err != nil {
        return nil
    }
    r := new( string )
    *r = string( text )
    return r
}

func LoadPages( filename string ) ( *Pages ) {
    text := Load( filename )
    if text == nil {
        return nil
    }
    pages := new( Pages )
    slideText := strings.Split( string( *text ), "---" )
    for _, body := range slideText {
        page  := Parse( filename, body )
        *pages = append( *pages, page )
    }
    return pages
}

func metadata( line string, splitText string ) ( string ) {
    if strings.Index( line, splitText ) == 0 {
        return strings.Trim( line[len( splitText ): ], " " )
    }
    return ""
}

func memberOf( text string, search string ) bool {
    return strings.Index( text, search ) == 0
}

func isIn( text string, search string ) bool {
    return strings.Index( text, search ) >= 0
}

func Segue( text string ) bool {
    return isIn( metadata( text, classSplit ), segueText )
}

// Use markdown descriptive text to parse text for HTML tag presentation
func Metadata( line string, splitText string, out* string, found *bool ) {
    if strings.Index( line, splitText ) == 0 {
        tmp   := strings.Trim( line[len( splitText ): ], " " )
        *out   = tmp
        *found = true
    }
}

func IsSet( text string, search string, out* bool, found* bool ) {
    if strings.Index( text, search ) == 0 && 
        strings.Index( text[ len( search ):], "true" ) > 0 {
        *out   = true
        *found = true
    }
}

func IsMarkdown( filename string ) bool {
    if debug { fmt.Println( "len( filename ) >= 3 && filename[len( filename )-3:] == \".md\"",  len( filename ) >= 3 && filename[len( filename )-3:] == ".md" ) }
    return len( filename ) >= 3 && filename[len( filename )-3:] == ".md"
}

func IsWiki( filename string ) bool {
    if debug { fmt.Println( "len( filename ) >= 5 && filename[len( filename )-5:] == \".wiki\"",  len( filename ) >= 5 && filename[len( filename )-5:] == ".wiki" ) }
    return len( filename ) >= 5 && filename[len( filename )-5:] == ".wiki"
}

func IsHTML( filename string ) bool {
    return len( filename ) >= 4 && filename[len( filename )-4:] == ".htm" || 
        len( filename ) >= 5 && filename[len( filename )-5:] == ".html" 
}

func Cut( text string, n int ) string {
    if len( text ) >= n {
        return text[:n]
    } else { return text }

}

func Split( text string ) string {
    if len(text) > 0 {
        return strings.Split( text, " " )[0]
    } else { return text }
}

var Fmap = template.FuncMap {
    "segue"     : Segue,
    "isMarkdown": IsMarkdown,
    "isWiki"    : IsWiki,
    "isHtml"    : IsHTML,
    "Cut"       : Cut,
    "Split"     : Split,
}

func MarkupMarkdown( filename string, args Arglist, raw bool ) string {
    if raw {
        text := Load( filename )
        pages := strings.Split( string( *text ), "---" ) 
        html  := ""
        for _, body := range pages {
            page  := Raw( filename, body )
            html  += page // append( html, page )
        }
        if text != nil {
            return fmt.Sprintf( "<!DOCTYPE html>\n<html>\n\n<title>\n   %s\n</title>\n<body>\n", filename ) + 
                string( blackfriday.MarkdownCommon( []byte( html  ) ) ) +
                "\n</body>\n</html>\n"
        } else { return "" }
	}

	if debug { fmt.Println( "MarkupMarkdown not raw",  filename ) }
	if IsWiki( filename ) { // len( filename ) >= 5 && filename[ len( filename ) - 5: ] == ".wiki" {
		fmt.Println( "MarkupMarkdown wiki",  filename )
        text := Load( filename )
		if output, err := wiki.Transform( *text ); err == nil {
			return output
		} else {
			return ""
		}
	}

	if debug { fmt.Println( "MarkupMarkdown not wiki",  filename ) }

    Head         := Load( *args.TemplateName + "-head.html" )
    Body         := Load( *args.TemplateName + "-body.html" )
    Foot         := Load( *args.TemplateName + "-foot.html" )

    if Body != nil && Head != nil && Foot != nil {
        runner, err := template.New("tmplutil").Funcs( Fmap ).Parse( *Body )
        if err != nil {
            fmt.Printf("parse error: %v\n", err)
            return ""
        }

        args.Pages   = LoadPages( filename )
        if args.Pages == nil {
            return ""
        }

        writebuffer := bytes.NewBuffer( make([]byte, 0 ) )
        writer      := bufio.NewWriter( writebuffer )
        err          = runner.Execute ( writer, args )

        if err != nil {
            fmt.Printf( "template evaluation error: %v\n", err )
            return ""
        }

        writer.Flush()
        return fmt.Sprintf( "%s\n%s\n%s\n", *Head, writebuffer.String(), *Foot )
    }
    return ""
}

func MakeHandler( fn func( http.ResponseWriter, *http.Request, string, string, Arglist ), args Arglist ) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
		Info.Printf( fmt.Sprintf( "Host %-20s Client %-20s URL.Path [%s] %s\n", r.Host, r.RemoteAddr, r.URL.Path, LogString( r ) ) )
        patharray := FilePathRegex.FindStringSubmatch( r.URL.Path )
		if patharray == nil {
			http.FileServer( http.Dir( *args.WebRoot ) )
			return
		}
        if patharray != nil && len( patharray ) > 3 {
			path, filename := patharray[2], patharray[3]
			if debug { fmt.Println( r.URL.Path, patharray, "filename", filename ) }
            args.Title = & filename
            fn( w, r, path, filename, args )
        // } else if len( r.URL.Path ) >= 5 && r.URL.Path[ len( r.URL.Path ) - 5: ] == ".wiki" {
        //     // m := WikiPathRegex.FindStringSubmatch( r.URL.Path )
		// 	m := patharray
		// 	if debug { fmt.Println( r.URL.Path, m ) }
        //     if m == nil {
        //         http.FileServer( http.Dir( *args.WebRoot ) )
        //         return
        //     }
        //     args.Title = & m[2]
		// 	WikiHandler( w, r, m[1], m[2], args )
        } else {
			http.FileServer( http.Dir( *args.WebRoot ) )
			// if debug { fmt.Println( r.URL.Path, patharray ) }
            // if patharray == nil {
            //     http.FileServer( http.Dir( *args.WebRoot ) )
            //     return
            // }
            // args.Title = & patharray[2]
            // fn( w, r, patharray[1], m[2], args )
        }
    }
}

func WrapHeadAndFoot( filename, html string ) string {
	return fmt.Sprintf( "<!DOCTYPE html>\n<html>\n\n<title>\n   %s\n</title>\n<body>\n %s\n</body>\n</html>\n", filename, html )
}

func WrapPre( filename, html string ) template.HTML {
	return template.HTML( fmt.Sprintf( "<!DOCTYPE html>\n<html>\n\n<title>\n   %s\n</title>\n<body><pre>\n %s\n</pre></body>\n</html>\n", filename, html ) )
}

func WikiHandler( w http.ResponseWriter, r *http.Request, path string, filename string, args Arglist ) {
	if IsWiki( filename ) {
        text := Load( filename )
		if text != nil {
			if t, err := wiki.Transform( *text ); err != nil {
				*text = ""
			} else {
				*text = WrapHeadAndFoot( filename, t )
			}
		}
		w.Write( []byte( template.HTML( *text ) ) )
	}
}

func Handler( w http.ResponseWriter, r *http.Request, path string, filename string, args Arglist ) {
	text := ""
	if debug { fmt.Println( filename ) }
	if IsMarkdown( filename ) {
		if debug { fmt.Println( "Handler IsMarkdown",  filename ) }
		text = MarkupMarkdown( filename, args, *args.TemplateName == "plain"  )
	} else {
		t := Load( filename )
		if t != nil {
			text = *t 
		}
	}
    if text == "" {
        http.NotFound( w, r )
        return
    }
    w.Write( []byte( template.HTML( text ) ) )
}
