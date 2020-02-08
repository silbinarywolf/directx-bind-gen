# DirectX Binding Generator

**WARNING:** This is an **unstable work-in-progress** project and not ready-for-use. If this project is of interest to you, please express that here on this [Github Issue](https://github.com/silbinarywolf/directx-bind-gen/issues/1).

## Introduction

This project aims to parse DirectX header files and provide a consumable set of data representing enums, functions, etc so that you can generate bindings for your programming language of choice.

My personal aim is to at least get DirectX11 bindings working in Golang but in all honesty, I might just give up on pursuing this further. It's tedious work and not exciting to do on-top of a day-job.

## What format will the consumable data be in?

Currently I export JSON files to the [data](data) folder of this repo. If there is an easier to consume format and there exists a Go Library that makes it frictionless to output to that format, I'll consider adding it.
