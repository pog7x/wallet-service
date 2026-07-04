// Package main is the entry point for the application.
package main

import "fmt"

type D struct {
	str  string
	list []int
}

func main() {
	d := D{str: "abc", list: []int{1, 2, 3}}

	d.str = "huy"
	d.list[0] = 5
	// d.list[0] = 5

	fmt.Println(d)
}
