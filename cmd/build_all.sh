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

go fmt $GOPATH/src/aletheiaware.com/spaceclientgo/...
go test $GOPATH/src/aletheiaware.com/spaceclientgo/...
env GOOS=darwin GOARCH=amd64 go build -o $GOPATH/bin/spaceclientgo-darwin-amd64 aletheiaware.com/spaceclientgo/cmd
# TODO env GOOS=darwin GOARCH=arm64 go build -o $GOPATH/bin/spaceclientgo-darwin-arm64 aletheiaware.com/spaceclientgo/cmd
env GOOS=linux GOARCH=386 go build -o $GOPATH/bin/spaceclientgo-linux-386 aletheiaware.com/spaceclientgo/cmd
env GOOS=linux GOARCH=amd64 go build -o $GOPATH/bin/spaceclientgo-linux-amd64 aletheiaware.com/spaceclientgo/cmd
env GOOS=linux GOARCH=arm GOARM=5 go build -o $GOPATH/bin/spaceclientgo-linux-arm5 aletheiaware.com/spaceclientgo/cmd
env GOOS=linux GOARCH=arm GOARM=6 go build -o $GOPATH/bin/spaceclientgo-linux-arm6 aletheiaware.com/spaceclientgo/cmd
env GOOS=linux GOARCH=arm GOARM=7 go build -o $GOPATH/bin/spaceclientgo-linux-arm7 aletheiaware.com/spaceclientgo/cmd
env GOOS=linux GOARCH=arm64 go build -o $GOPATH/bin/spaceclientgo-linux-arm8 aletheiaware.com/spaceclientgo/cmd
env GOOS=windows GOARCH=386 go build -o $GOPATH/bin/spaceclientgo-windows-386.exe aletheiaware.com/spaceclientgo/cmd
env GOOS=windows GOARCH=amd64 go build -o $GOPATH/bin/spaceclientgo-windows-amd64.exe aletheiaware.com/spaceclientgo/cmd
