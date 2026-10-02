# pqcota (통합 리포) — 빌드·테스트·게이트
# 전제: go(go.mod의 go 줄 이상 — 지금은 1.26.4. 빌드는 toolchain 줄의 1.26.6으로 돈다). 형제 리포 넷(pqcota-common·-inventory·-discovery·-provisioning)을
# 이 리포와 **나란히** 클론해 둔다. go.mod가 replace로 ../ 를 읽고, 게이트가 그 넷을 함께 잰다.
#
# 이 리포에는 코드가 거의 없다(tools·데모 소품·리포 사이 통합 테스트). 계약과 생성 코드는 pqcota-common에 있고,
# `make generate`·`lint`·`breaking`은 거기서 돈다. 여기서는 다섯 리포를 가로지르는 것만 잰다.

SIBLINGS  := ../pqcota-common ../pqcota-inventory ../pqcota-discovery ../pqcota-provisioning
DISCOVERY := ../pqcota-discovery

.PHONY: all repos fmt-check build build-jar test vet check-boundary check-docs check-collectors check-gates check-deps check-prose

all: repos fmt-check check-boundary check-docs check-collectors check-gates check-deps check-prose vet build build-jar test

# 형제 리포 넷의 자기 게이트. 통합이 초록이어도 넷 가운데 하나가 빨간 채로 나가지 않게 한다.
repos:
	@for r in $(SIBLINGS); do echo "▶ $$r"; $(MAKE) -C $$r || exit 1; done

# gofmt 게이트 — CONTRIBUTING이 gofmt를 규정하는데 검사가 없어 미포맷이 8건까지 쌓인 적이 있다.
# gen/(생성 코드)은 제외. 실패 시 어떤 파일인지 보여준다.
fmt-check:
	@files=$$(gofmt -l $$(git ls-files '*.go' | grep -v '^gen/') 2>/dev/null); \
	if [ -n "$$files" ]; then \
	  echo "✗ gofmt 필요:"; echo "$$files"; echo "  고치기: gofmt -w <파일>"; exit 1; \
	fi; \
	echo "✓ gofmt 통과"

# 이 리포의 Go 코드(tools·데모 소품·통합 테스트) 빌드 — 호스트 + 리눅스 + Windows.
# collector 같은 배포 명령의 교차 빌드는 그 코드가 있는 pqcota-discovery의 `make build`가 한다.
#
# ★ 리눅스 타깃을 따로 빌드하는 이유: collector의 핵심(`/proc`·AF_PACKET·attach)은 `//go:build linux`라
# **macOS에서는 컴파일 대상에서 빠진다.** 호스트 빌드만 하면 Mac 기여자가 그 코드를 깨도 통과한다.
# 교차 컴파일이 공짜(CGO_ENABLED=0)라 늘 함께 확인한다.
build:
	go build ./...
	@echo "→ 리눅스 타깃 교차 확인(리눅스 전용 파일 포함)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dev/null ./... 2>&1 | head -20; \
	 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dev/null ./... >/dev/null
	@# Windows 타깃도 함께 본다 — CNG collector가 여기서 자란다. 리눅스 전용 코드가
	@# 빌드 태그 밖으로 새면 **Windows에서만** 깨지므로, 그 코드를 쓰기 전에 게이트를 세운다.
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /dev/null ./... 2>&1 | head -20; \
	 CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /dev/null ./... >/dev/null
	@echo "✓ Go 빌드(호스트 + linux/amd64 + windows/amd64) 통과"

# Java attach 사이드카 — 소스는 pqcota-discovery에 있고, 거기서 빌드해 산출물을 이 리포의 build/로 가져온다.
# JDK가 없으면 건너뛰되 알리지 않고 넘기지 않는다(pqcota-discovery의 build-jar가 그 경고를 낸다).
build-jar:
	@$(MAKE) -C $(DISCOVERY) build-jar
	@if [ -f $(DISCOVERY)/build/collector.jar ]; then mkdir -p build && cp $(DISCOVERY)/build/collector.jar build/collector.jar && echo "✓ build/collector.jar"; fi

# 경계 표현 게이트 — 이 리포는 다른 티어를 **지목하지 않는다**. 여기 없는 기능은 "하지 않는다"로
# 적고, 계획은 로드맵에 적는다(위치 선언은 check-docs 규칙 (2)가 따로 막는다). 사람 기억에만
# 맡기면 새어 들어가므로 빌드에서 막는다.
#   허용 예외: "Community Edition"(이 리포 자신의 이름), "enterprise intranet"(기업 내부망의 영문).
#   EE는 단어 경계로만 — 영문 문서의 feed·between 같은 낱말에 걸리지 않게.
#   "CORE 트랙"류도 막는다 — 트랙을 나누는 순간 다른 트랙이 있다는 전제가 깔린다(실제로 새어 들었다).
check-boundary:
	@hits=$$( { grep -rnwE 'EE' --include='*.md' --include='*.go' \
	              --exclude-dir=.git --exclude-dir=gen . $(SIBLINGS) ; \
	            grep -rnE '[Ee]nterprise|상용|해자|moat|프리미엄|[Cc]ore 트랙|CORE track|core track' --include='*.md' --include='*.go' \
	              --exclude-dir=.git --exclude-dir=gen . $(SIBLINGS) ; } \
	          | grep -vE 'Community Edition|enterprise intranet' || true ); \
	if [ -n "$$hits" ]; then \
	  echo "✗ 다른 티어를 지목하는 표현이 있다 — 그 기능을 \"하지 않는다\"로 적을 것:"; \
	  echo "$$hits"; \
	  exit 1; \
	fi; \
	echo "✓ 경계 표현 검사 통과"

# 문서 게이트 — 링크·앵커 무결성 + 낡은 범위 표현 + 역할분담 산문 + 개인정보 + 라이선스 표 대조.
# 코드는 테스트가 지키는데 문서는 아무도 안 지켜서 아무도 모르는 사이에 낡는다. 여기서 막는다.
# 검사기는 Go다 — 이 리포를 빌드하려면 Go가 이미 필요하므로 새 런타임 전제가 없다.
# 리포마다 자기 안의 문서를 잰다(상대 링크는 리포 안에서만 성립한다). 검사기는 이 리포에 있고
# 형제 리포에서는 그 리포를 작업 디렉터리로 삼아 돌린다.
check-docs:
	@go build -o build/checkdocs ./tools/checkdocs
	@for r in . $(SIBLINGS); do (cd $$r && $(CURDIR)/build/checkdocs) || exit 1; done

# collector 목록 게이트 — 릴리스 워크플로가 빌드하는 것과 참조 플레이북이 노드로 반입하는 것이
# 같은지 본다. 플레이북은 pqcota-discovery에 있고 워크플로는 이 리포에 있다.
# 같은 목록을 두 곳에 두고 있어서 한쪽만 바뀌면 **받아서 돌릴 때** 드러난다.
# v0.6.3에서 실제로 갈라졌다(플레이북에 pqcota-jvmscan을 더하고 워크플로를 안 고쳤다).
check-collectors:
	@go build -o build/checkcollectors ./tools/checkcollectors && ./build/checkcollectors $(DISCOVERY)

# 게이트 배선 검사 — 규칙을 적어 두고 제품이 부르지 않으면 보장이 아니다. 실제로 pqcota-provision이
# provisioning.Executable을 부르지 않는 동안 승인 서명이 빈 계획이 통과했다. 테스트는 규칙이 옳은지
# 보지, 그 규칙이 쓰이는지 보지 않는다.
#   배선은 리포를 가로지른다: 게이트 함수는 pqcota-common·-provisioning에 있고 그것을 부르는 명령은
#   pqcota-inventory·-discovery·-provisioning에 있어서, 다섯 리포를 함께 읽는다.
#   같은 부류로 **규칙 판 자리표시자**도 막는다. normalize.RulesetVersion 하나가 파생값의 근거를
#   말하기로 해 놓고 적재 명령이 "ruleset-demo"를 넘기면, 아무것도 실패하지 않은 채 이력 비교만
#   아무 표시 없이 무의미해진다 — 모든 스냅샷이 같은 자리표시자를 달기 때문이다.
check-gates:
	@go build -o build/checkgates ./tools/checkgates && ./build/checkgates . $(SIBLINGS)

# 단계 사이의 import 방향(인벤토리가 허브, collector는 common과 procs만). 규칙은 tools/checkdeps/rules.tsv.
# 다섯 리포를 한꺼번에 잰다 — 형제 모듈을 가리키는 import도 규칙표의 영역으로 분류한다.
check-deps:
	@go build -o build/checkdeps ./tools/checkdeps && ./build/checkdeps . $(SIBLINGS)

# 문체 게이트 — 문서·HTML·도구 출력의 한국어에서 **한 번 걷어낸 말이 다시 들어오지 않게** 한다.
# 지금 있는 것은 tools/checkprose/baseline.tsv에 파일마다 적어 두고 늘면 막는다. 고쳐서 줄었으면
# `go run ./tools/checkprose -baseline`으로 기준선을 내려 같은 커밋에 넣는다.
check-prose:
	@go run ./tools/checkprose

vet:
	go vet ./...

test:
	go test ./...
