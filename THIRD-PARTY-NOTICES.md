한국어 · [English](THIRD-PARTY-NOTICES.en.md)

# Third-Party Notices

이 리포를 빌드해 만든 바이너리(collector·CLI)에 **컴파일·링크되는** 서드파티 컴포넌트의 저작권·
라이선스 고지입니다. 전부 허용적 라이선스(Apache-2.0 / BSD-3-Clause / MIT)이며, 그 바이너리를 남에게
전달할 때 저작권 고지를 유지해야 하므로 아래에 원문을 둡니다.

> 빌드 타임 도구(buf·protoc-gen-*)와 데모 환경 구성요소(Ansible·Temurin·OpenSSL·Graphviz 등, 별도 프로세스라
> 링크되지 않음)를 포함한 **전체 라이선스 지형**은 [docs/licensing.md](docs/licensing.md)를 참조하세요.

---

## Apache License 2.0

동일 라이선스가 이 프로젝트의 [`LICENSE`](LICENSE)에 전문으로 포함되어 있습니다. 다음 컴포넌트가 이를 따릅니다:

- **google.golang.org/grpc** — Copyright The gRPC Authors (Google LLC 외)
- **google.golang.org/genproto/googleapis/rpc** — Copyright Google LLC

전문: [`LICENSE`](LICENSE) 또는 https://www.apache.org/licenses/LICENSE-2.0

---

## BSD 3-Clause "New" License

다음 컴포넌트가 이를 따릅니다:

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

다음 컴포넌트가 이를 따릅니다 (모두 Jack Christensen, jackc/pgx 생태계):

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

_이 목록은 `go.mod`의 배포 대상 의존성 기준입니다. 의존성 갱신 시 `go-licenses report ./...` 등으로
재생성하는 것을 권장합니다. 데모 전용·빌드 전용 컴포넌트는 링크되지 않으므로 여기 포함하지 않습니다._
