package test

import (
  "log"
  "os"
  "path"
  "strings"
  "golang.org/x/net/html"
  "net/url"
  "net/http"
  "github.com/wjdp/htmltest/issues"
  "github.com/wjdp/htmltest/doc"
  "github.com/wjdp/htmltest/refcache"
)

func CheckLink(document *doc.Document, node *html.Node) {
  attrs := extractAttrs(node.Attr, []string{"href", "rel", "data-proofer-ignore"})

  // Do not check canonical links
  if attrs["rel"] == "canonical" { return }
  // Ignore if data-proofer-ignore set
  if attrPresent(node.Attr, "data-proofer-ignore") { return }

  if _, ok := attrs["href"]; ok {
    ref := doc.NewReference(document, node, attrs["href"])
    switch ref.Scheme {
    case "http":
      if Opts.EnforceHTTPS {
        issues.AddIssue(issues.Issue{
          Level: issues.ERROR,
          Message: "is not an HTTPS link",
          Reference: ref,
          })
      }
      CheckExternal(ref)
    case "https":
      CheckExternal(ref)
    case "file":
      CheckInternal(ref)
    case "mailto":
    case "tel":

    }
  } else {
    issues.AddIssue(issues.Issue{
      Level: issues.DEBUG,
      Message: "anchor without href",
      Document: document,
    })
  }
}

func CheckExternal(ref *doc.Reference) {
  if !Opts.CheckExternal {
    issues.AddIssue(issues.Issue{
      Level: issues.DEBUG,
      Message: "skipping",
      Reference: ref,
    })
    return
  }

  urlStr := doc.URLString(ref)
  if Opts.StripQueryString && !InList(Opts.StripQueryExcludes, urlStr) {
    urlStr = doc.URLStripQueryString(urlStr)
  }
  var statusCode int

  if refcache.CachedURLStatus(urlStr) != 0 {
    // If we have the result in cache, return that
    statusCode = refcache.CachedURLStatus(urlStr)
  } else {
    // log.Println("Ext", ref.Document.Path, doc.URLString(ref))
    urlUrl, err := url.Parse(urlStr)
    req := &http.Request{
      Method: "GET",
      URL: urlUrl,
      Header: map[string][]string{
        "Range": {"bytes=0-63"}, // If server supports prevents body being sent
      },
    }
    _ = req
    resp, err := httpClient.Do(req)
    // resp, err := httpClient.Get(urlStr)

    if err != nil {
      if strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers") {
        issues.AddIssue(issues.Issue{
          Level: issues.ERROR,
          Message: "Request timed out",
          Reference: ref,
        })
        return
      }
      if strings.Contains(err.Error(), "no such host") {
        issues.AddIssue(issues.Issue{
          Level: issues.ERROR,
          Message: "no such host",
          Reference: ref,
        })
        return
      }
      if strings.Contains(err.Error(), "write on closed buffer") {
        issues.AddIssue(issues.Issue{
          Level: issues.ERROR,
          Message: err.Error(),
          Reference: ref,
        })
        return
      }
      log.Fatal("Unhandled httpClient error: " + err.Error())
    }
    // Save cached result
    refcache.SetCachedURLStatus(urlStr, resp.StatusCode)
    statusCode = resp.StatusCode
    if statusCode == 200 { log.Println(urlStr) }
  }

  switch statusCode {
  case http.StatusOK://, http.StatusPartialContent:
    issues.AddIssue(issues.Issue{
      Level: issues.DEBUG,
      Message: http.StatusText(statusCode),
      Reference: ref,
    })
  case http.StatusPartialContent:
    issues.AddIssue(issues.Issue{
      Level: issues.INFO,
      Message: http.StatusText(statusCode),
      Reference: ref,
    })
  default:
    log.Println(urlStr)
    issues.AddIssue(issues.Issue{
      Level: issues.ERROR,
      Message: http.StatusText(statusCode),
      Reference: ref,
    })
  }

  // TODO check a hash id exists in external page if present in reference (URL.Fragment)

}

func CheckInternal(ref *doc.Reference) {
  if !Opts.CheckInternal {
    issues.AddIssue(issues.Issue{
      Level: issues.DEBUG,
      Message: "skipping",
      Reference: ref,
    })
    return
  }
  // log.Println("CheckInternal", ref.Document.Path, doc.AbsolutePath(ref))

  fPath := makePath(doc.AbsolutePath(ref))
  CheckFile(ref, fPath)
}

func CheckFile(ref *doc.Reference, fPath string) {
  f, err := os.Stat(fPath)
  if os.IsNotExist(err) {
    issues.AddIssue(issues.Issue{
      Level: issues.ERROR,
      Message: "target does not exist",
      Reference: ref,
    })
    return
  }
  checkErr(err) // Crash on other errors

  if f.IsDir() {
    issues.AddIssue(issues.Issue{
      Level: issues.DEBUG,
      Message: "target is a directory",
      Reference: ref,
    })
    CheckFile(ref, path.Join(fPath, Opts.DirectoryIndex))
    return
  }
}
