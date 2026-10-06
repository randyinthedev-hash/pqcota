// SPDX-FileCopyrightText: 2026 randyinthedev
// SPDX-License-Identifier: Apache-2.0

// Package crossstage — 여러 단계의 패키지를 함께 import하는 통합 테스트만 두는 자리.
//
// 어느 한 단계의 소유가 아닌 테스트(예: collector와 선언 임포터가 낸 결과의 계약 불변식, 저장소 구현체
// 전부가 org.Scoped를 만족하는지)는 대상 코드 옆에 둘 수 없다. 옆에 두면 그 패키지가 다른 단계를
// import하게 되어 단계 의존 방향이 깨진다. 그래서 이 디렉터리 한 곳에 모은다.
//
// 이 디렉터리에는 이 파일(패키지 문서와 선언뿐)과 _test.go만 둔다. 구현 코드를 두지 않는다.
package crossstage
