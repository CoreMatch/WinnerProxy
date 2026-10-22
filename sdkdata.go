package main

import "embed"

//go:embed sdk/*
var sdkSource embed.FS
