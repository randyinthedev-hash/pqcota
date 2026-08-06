# pqcota — 빌드·테스트
# 전제: go(>=1.24), buf, protoc-gen-go 설치. PATH에 $GOPATH/bin 포함(`make tools`).
#
# gen/ (proto 생성 코드)은 gitignore 대상 — 클론 후 `make generate` 필수.

.PHONY: all generate lint breaking fmt-check build build-jar test vet tools check-boundary check-docs

all: generate lint breaking fmt-check check-boundary check-docs vet build build-jar test

# 전체 빌드 — Go(호스트 + **리눅스 타깃**) + Java 사이드카.
#
# ★ 리눅스 타깃을 따로 빌드하는 이유: collector의 핵심(`/proc`·AF_PACKET·attach)은 `//go:build linux`라
# **macOS에서는 컴파일 대상에서 빠진다.** 호스트 빌드만 하면 Mac 기여자가 그 코드를 깨도 통과한다.
# 교차 컴파일이 공짜(CGO_ENABLED=0)라 늘 함께 확인한다.
build:
	go build ./...
	@echo "→ 리눅스 타깃 교차 확인(리눅스 전용 파일 포함)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dev/null ./... 2>&1 | head -20; \
	 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dev/null ./... >/dev/null
	@echo "✓ Go 빌드(호스트 + linux/amd64) 통과"

# Java attach 사이드카 — JDK가 없으면 **건너뛰되 조용히 넘기지 않는다**(§2.6 결).
# 산출물은 build/collector.jar. 데모는 컨테이너 안에서 같은 걸 빌드한다.
#
# ★ 두 번 컴파일하는 이유 — **대상 JVM의 하한이 곧 관측 커버리지**다. 관측 대상 안으로 들어가는 것은
# 에이전트(IntrospectAgent)뿐이고 그것은 Java 8 API만 쓰므로 `--release 8`로 낮춘다. Attacher는
# `com.sun.tools.attach`(JDK 9+ 모듈)를 써서 8로 못 낮추지만, 대상 JVM 안에서 로드되지 않으므로
# 상관없다. 한 번에 컴파일하면 전부 빌드 JDK의 클래스 버전이 되어 **낡은 JVM에서 로드조차 안 된다**
# (실측: JDK 21로 만든 jar은 17·11·8에서 LinkageError).
JVM_COLLECTOR := discovery/collectors/jvm/collector
build-jar:
	@if ! command -v javac >/dev/null; then \
	  echo "⚠ javac 없음 — Java 사이드카 빌드 건너뜀(JDK 11+ 필요). collector.jar가 없으면 attach 경로를 쓸 수 없다."; \
	  exit 0; \
	fi; \
	rm -rf build/jvmcls && mkdir -p build/jvmcls && \
	javac --release 8 -nowarn -d build/jvmcls \
	  $(JVM_COLLECTOR)/src/main/java/pqcota/jvm/IntrospectAgent.java && \
	javac --release 11 -nowarn -d build/jvmcls \
	  $(JVM_COLLECTOR)/src/main/java/pqcota/jvm/Attacher.java \
	  $(JVM_COLLECTOR)/src/main/java/pqcota/jvm/StaticFallback.java && \
	jar cfm build/collector.jar $(JVM_COLLECTOR)/manifest.mf -C build/jvmcls . && \
	rm -rf build/jvmcls && \
	echo "✓ Java 사이드카: build/collector.jar"

# gofmt 게이트 — CONTRIBUTING이 gofmt를 규정하는데 검사가 없어 미포맷이 8건까지 쌓인 적이 있다.
# gen/(생성 코드)은 제외. 실패 시 어떤 파일인지 보여준다.
fmt-check:
	@files=$$(gofmt -l $$(git ls-files '*.go' | grep -v '^gen/') 2>/dev/null); \
	if [ -n "$$files" ]; then \
	  echo "✗ gofmt 필요:"; echo "$$files"; echo "  고치기: gofmt -w <파일>"; exit 1; \
	fi; \
	echo "✓ gofmt 통과"

# proto SSOT → Go 코드 생성 (gen/pqcota/{common,discovery,provisioning}/v1/*.pb.go)
# gen/은 커밋하지 않으므로 클론 직후 필수다. buf가 없으면 무엇을 설치해야 하는지 알려준다 —
# "command not found"만 보이면 원인이 컨트랙트인지 도구인지 알 수 없다.
generate:
	@command -v buf >/dev/null || { \
	  echo "✗ buf 없음 — proto 코드 생성 도구가 필요하다: https://buf.build/docs/installation"; \
	  echo "  설치 후: make tools && make generate"; exit 1; }
	@# buf는 플러그인을 **PATH에서** 찾는다. `make tools`가 방금 깔았어도 $$(go env GOPATH)/bin이
	@# PATH에 없으면 "executable file not found"만 나와, 읽는 사람은 설치가 실패한 줄 안다.
	@# 깔려 있는데 안 보이는 것인지, 아예 없는 것인지를 갈라서 말한다.
	@for p in protoc-gen-go protoc-gen-go-grpc; do \
	  command -v $$p >/dev/null && continue; \
	  if [ -x "$$(go env GOPATH 2>/dev/null)/bin/$$p" ]; then \
	    echo "✗ $$p 이 PATH에 없다 — 설치는 돼 있다($$(go env GOPATH)/bin)."; \
	    echo "  buf는 플러그인을 PATH에서 찾으므로 그 디렉터리를 PATH에 넣어야 한다:"; \
	    echo "    export PATH=\"\$$PATH:\$$(go env GOPATH)/bin\"     # zsh·bash — 셸 설정에 넣어 두면 매번 안 해도 된다"; \
	    echo "    \$$env:Path += \";\$$(go env GOPATH)/bin\"          # Windows PowerShell (정방향 슬래시도 받는다)"; \
	  else \
	    echo "✗ $$p 없음 — 먼저 make tools 를 실행한다."; \
	  fi; exit 1; \
	done
	cd contracts && buf generate

# 계약 lint (STANDARD, 일부 오피니언 규칙 완화 — buf.yaml)
lint:
	@command -v buf >/dev/null || { echo "✗ buf 없음 — https://buf.build/docs/installation"; exit 1; }
	cd contracts && buf lint

# 계약 하위호환 — **이미 릴리스한 계약**을 깨지 않는지 본다(buf.yaml의 `breaking: FILE`).
#
# 기준선은 마지막 릴리스 태그다. 직전 커밋이 아니라 태그인 이유: 릴리스 전에는 계약을 다시
# 짜는 것이 정당하고, 받는 사람이 생긴 뒤부터 못 깨는 것이다. "새 런타임을 코어 무변경으로
# 받는다"는 주장도 **published 계약 기준**이라야 뜻이 있다.
# 태그가 없으면(v0.1.0 이전) 비교 대상이 없다 — 건너뛰되 **왜 건너뛰는지 찍는다**(스킵은 통과가 아니다).
breaking:
	@command -v buf >/dev/null || { echo "✗ buf 없음 — https://buf.build/docs/installation"; exit 1; }
	@if [ -n "$(AGAINST)" ]; then \
	  echo "· 계약 하위호환 기준선: $(AGAINST) (branch)"; \
	  cd contracts && buf breaking --against "../.git#branch=$(AGAINST),subdir=contracts"; \
	  exit $$?; \
	fi; \
	tag=$$(git tag -l 'v*' --sort=-v:refname | head -1); \
	if [ -z "$$tag" ]; then \
	  echo "· 계약 하위호환 검사 건너뜀 — 릴리스 태그가 아직 없다(첫 태그부터 기준선이 생긴다)"; \
	  echo "  작업 중 브랜치를 main과 대조하려면: make breaking AGAINST=main"; \
	else \
	  echo "· 계약 하위호환 기준선: $$tag"; \
	  cd contracts && buf breaking --against "../.git#tag=$$tag,subdir=contracts"; \
	fi

# 경계 표현 게이트 — 이 리포는 다른 티어를 **지목하지 않는다**. 여기 없는 기능은 "하지 않는다"로
# 적고, 계획은 로드맵이 말한다(위치 선언은 check-docs 규칙 (2)가 따로 막는다). 사람 기억에만
# 맡기면 새어 들어가므로 빌드에서 막는다.
#   허용 예외: "Community Edition"(이 리포 자신의 이름), "enterprise intranet"(기업 내부망의 영문).
#   EE는 단어 경계로만 — 영문 문서의 feed·between 같은 낱말에 걸리지 않게.
check-boundary:
	@hits=$$( { grep -rnwE 'EE' --include='*.md' --include='*.go' \
	              --exclude-dir=.git --exclude-dir=gen . ; \
	            grep -rnE '[Ee]nterprise|상용|해자|moat|프리미엄' --include='*.md' --include='*.go' \
	              --exclude-dir=.git --exclude-dir=gen . ; } \
	          | grep -vE 'Community Edition|enterprise intranet' || true ); \
	if [ -n "$$hits" ]; then \
	  echo "✗ 다른 티어를 지목하는 표현이 있다 — 그 기능을 \"하지 않는다\"로 적을 것:"; \
	  echo "$$hits"; \
	  exit 1; \
	fi; \
	echo "✓ 경계 표현 검사 통과"

# 문서 게이트 — 링크·앵커 무결성 + 낡은 범위 표현 + 역할분담 산문 + 개인정보 + 라이선스 표 대조.
# 코드는 테스트가 지키는데 문서는 아무도 안 지켜서 조용히 썩는다. 여기서 막는다.
# 검사기는 Go다 — 이 리포를 빌드하려면 Go가 이미 필요하므로 새 런타임 전제가 없다(§2.4).
# go run 대신 빌드해서 실행: go run은 실패 시 "exit status 1"을 덧붙여 게이트 출력이 지저분해진다.
check-docs:
	@go build -o build/checkdocs ./tools/checkdocs && ./build/checkdocs

vet:
	go vet ./...

test:
	go test ./...

# 도구 설치 헬퍼 (buf는 릴리스 바이너리 권장)
# 어디에 깔리는지 함께 알린다 — go install은 조용히 $(go env GOPATH)/bin에 넣는데, 거기가 PATH에
# 없으면 다음 단계인 make generate가 "플러그인 없음"으로 넘어져 원인이 설치처럼 보인다.
#
# **버전을 고정한다.** `@latest`면 언제 깔았느냐로 생성 코드가 달라져, 같은 커밋에서 사람마다
# 다른 gen/이 나온다(gen/은 커밋하지 않으므로 더 드러나지 않는다). PROTOC_GEN_GO는 go.mod의
# google.golang.org/protobuf와 **같은 버전**이어야 한다 — 생성 코드가 그 런타임을 부른다.
# 데모의 ctl 이미지도 같은 값으로 깐다(demo/Dockerfile).
PROTOC_GEN_GO      := v1.36.11
PROTOC_GEN_GO_GRPC := v1.5.1
tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC)
	@echo "✓ 설치 위치: $$(go env GOPATH)/bin"
	@command -v protoc-gen-go >/dev/null \
	  || echo "  ⚠ 이 디렉터리가 PATH에 없다 — make generate 전에 넣어야 한다: export PATH=\"\$$PATH:\$$(go env GOPATH)/bin\""
