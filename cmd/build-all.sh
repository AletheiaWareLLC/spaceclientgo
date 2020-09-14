#!/bin/bash
#
# Copyright 2019 Aletheia Ware LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e
set -x

go fmt $GOPATH/src/github.com/AletheiaWareLLC/spaceclientgo/...
go test $GOPATH/src/github.com/AletheiaWareLLC/spaceclientgo/...
env GOOS=darwin GOARCH=386 go build -o $GOPATH/bin/spaceclientgo-darwin-386 github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=darwin GOARCH=amd64 go build -o $GOPATH/bin/spaceclientgo-darwin-amd64 github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=linux GOARCH=386 go build -o $GOPATH/bin/spaceclientgo-linux-386 github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=linux GOARCH=amd64 go build -o $GOPATH/bin/spaceclientgo-linux-amd64 github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=linux GOARCH=arm GOARM=5 go build -o $GOPATH/bin/spaceclientgo-linux-arm5 github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=linux GOARCH=arm GOARM=6 go build -o $GOPATH/bin/spaceclientgo-linux-arm6 github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=linux GOARCH=arm GOARM=7 go build -o $GOPATH/bin/spaceclientgo-linux-arm7 github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=linux GOARCH=arm64 go build -o $GOPATH/bin/spaceclientgo-linux-arm8 github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=windows GOARCH=386 go build -o $GOPATH/bin/spaceclientgo-windows-386.exe github.com/AletheiaWareLLC/spaceclientgo/cmd
env GOOS=windows GOARCH=amd64 go build -o $GOPATH/bin/spaceclientgo-windows-amd64.exe github.com/AletheiaWareLLC/spaceclientgo/cmd
