package main

import (
	"fmt"

	"example.com/stringutil"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func main() {
	reversed := stringutil.Reverse("Hello")
	titled := cases.Title(language.English).String(reversed)
	fmt.Print(titled)
}
