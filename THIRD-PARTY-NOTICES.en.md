English · [한국어](THIRD-PARTY-NOTICES.md)

# Third-Party Notices

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](THIRD-PARTY-NOTICES.md) is authoritative.

Copyright and license notices for the third-party components **compiled and linked into** the binaries you build from this repo (collectors and CLIs). All are permissive (Apache-2.0 / BSD-3-Clause / MIT); the notices are reproduced below because they must be preserved when you pass those binaries on.

> For the **full license landscape** — including build-time tools (buf · protoc-gen-*) and demo-environment components (Ansible · Temurin · OpenSSL · Graphviz, etc., which run as separate processes and are not linked) — see [docs/licensing.md](docs/licensing.en.md).

---

## Apache License 2.0

The same license is included in full as this project's [`LICENSE`](LICENSE). The following components are covered by it:

- **google.golang.org/grpc** — Copyright The gRPC Authors (Google LLC et al.)
- **google.golang.org/genproto/googleapis/rpc** — Copyright Google LLC

Full text: [`LICENSE`](LICENSE) or https://www.apache.org/licenses/LICENSE-2.0

---

## BSD 3-Clause "New" License

The following components are covered by it:

- **google.golang.org/protobuf** — Copyright (c) 2018 The Go Authors. All rights reserved.
- **golang.org/x/sys** — Copyright 2009 The Go Authors.
- **golang.org/x/net** — Copyright 2009 The Go Authors.
- **golang.org/x/sync** — Copyright 2009 The Go Authors.
- **golang.org/x/text** — Copyright 2009 The Go Authors.

```
Copyright (c) 2009 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

---

## MIT License

The following components are covered by it (all from Jack Christensen, the jackc/pgx ecosystem):

- **github.com/jackc/pgx/v5** — Copyright (c) 2013-2021 Jack Christensen
- **github.com/jackc/pgpassfile** — Copyright (c) 2019 Jack Christensen
- **github.com/jackc/pgservicefile** — Copyright (c) 2020 Jack Christensen
- **github.com/jackc/puddle/v2** — Copyright (c) 2018 Jack Christensen

```
MIT License

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

---

_This list reflects the distribution-target dependencies in `go.mod`. When dependencies change, regenerating with e.g. `go-licenses report ./...` is recommended. Demo-only and build-only components are not linked and are therefore not included here._
