package ggfetch

import (
	"bytes"
	"net/url"
	"strings"

	"code.google.com/p/go.net/html/atom"

	"code.google.com/p/go.net/html"
)

const escapedFragment = "_escaped_fragment_"

// This file implements Ajax Crawling according to:
// https://developers.google.com/webmasters/ajax-crawling/docs/specification

func escapeFragment(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil || !strings.HasPrefix(u.Fragment, "!") {
		return rawurl
	}
	q := u.Query()
	q.Set(escapedFragment, u.Fragment[1:])
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String()
}

func unescapeFragment(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return rawurl
	}
	q := u.Query()
	fragment := q.Get(escapedFragment)
	q.Del(escapedFragment)
	u.RawQuery = q.Encode()
	if fragment != "" {
		u.Fragment = "!" + fragment
	}
	return u.String()
}

func escapeFragmentMeta(rawurl string, content []byte) (newurl string, escaped bool) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return
	}

	q := u.Query()
	if _, ok := q[escapedFragment]; ok {
		return
	}

	// let's parse this document and find the fragment meta
	tokenizer := html.NewTokenizer(bytes.NewReader(content))
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return
		case html.SelfClosingTagToken:
			t := tokenizer.Token()
			if t.DataAtom == atom.Meta {
				var name, content string
				for _, attr := range t.Attr {
					switch attr.Key {
					case "name":
						name = attr.Val
					case "content":
						content = attr.Val
					}
				}
				if name == "fragment" && strings.HasPrefix(content, "!") {
					q.Set(escapedFragment, content[1:])
					u.RawQuery = q.Encode()
					return u.String(), true
				}
			}
		case html.StartTagToken:
			if tokenizer.Token().DataAtom == atom.Body {
				return
			}
		}
	}
}
