# 검증 기록

환경: Windows/amd64, Go 1.25.0. 실제 외부 도메인 스캔은 수행하지 않았습니다.

## 통과한 검증

- `go test ./... -count=1`: 모의 스캐너 실행 파일과 로컬 HTTP/API 서버를 사용하는 기본 자동화 테스트.
- `go vet ./...`: 정적 검사.
- `go build -o discoveryx.exe ./cmd/discoveryx`: Windows 실행 파일 빌드.
- `discoveryx.exe --help`, `--version`: PowerShell에서 확인. `--version`은 CMD에서도 확인.
- `tools install --tools-dir .validation/tools`: 공식 Windows/amd64 안정 릴리스 7개 다운로드, SHA256 검증, 실제 도움말과 버전 실행 확인.
- `tools list --tools-dir .validation/tools`, `doctor --tools-dir .validation/tools`: 저장된 버전, 설치 무결성과 의존성 진단 확인. 사전·템플릿·프로브 부재 및 SYN 권한 부족은 환경 상태로 표시됨.
- 선택적 `TestInstalledScannersOnLoopback`: 실제 Httpx 1.12.0, Katana 1.8.0, Feroxbuster 2.13.1을 로컬 HTTP 테스트 서버에 실행. 웹 서비스 1개와 중복 제거 후 엔드포인트 3개 정규화, 외부 호스트 링크 요청 0건.
- `GOOS=linux CGO_ENABLED=0 go build`: Linux/amd64 교차 컴파일. Linux에서의 실제 프로세스/스캐너 실행 검증은 하지 않음.
- 실제 Nuclei 3.11.1에 로컬 무서명 fixture를 대상으로 `-tl -dut`와 `-validate`만 수행. `-tl`이 무서명 파일도 표시할 수 있음을 확인해, 완료 판정에 실제 엔진 로드 수 검증을 추가함. 해당 명령은 스캔 대상 URL 없이 템플릿 목록/검증만 실행함.

## 기본 테스트 범위

CLI 단축/긴 옵션과 first/last/both/off 계획, web-only 우선순위, 단계별 결과 전달, 원본 도메인 유지, 신규 해석 IP만 마지막에 스캔, 기술별 확장자와 IIS 비추정, 비정상 JSONL, URL/호스트 중복 제거, 범위 경계·제외, SemVer/릴리스 자산 선택, 재설치 방지, 체크섬 오류, 업데이트 실패 시 Manifest·기존 파일 유지, 템플릿 정책·갱신 롤백·로드 0개 처리, timeout과 자식 프로세스 트리 종료, 한글/공백 경로, OS 잠금 재획득, 부분 실패 후 재개, 체크포인트 손상과 바이너리 변경 거부를 포함합니다.

## 0.2 변경 검증

- 포트 스캔 테스트는 DNS 결과로 초기/최종 단계를 실행하고, 최종 단계에서 이미 스캔한 IP가 제외되는지 확인합니다.
- Web-only 통합 테스트는 Httpx → Feroxbuster 전달, 단일 URL의 스킴·포트·경로·쿼리 유지, PHP 확장자 선택, 메타데이터 저장과 재개를 확인합니다. Httpx가 실패해도 원래 URL로 다음 웹 단계를 진행하는 경우를 포함합니다.
- 실제 Httpx 1.12.0으로 루프백 PHP 헤더 응답을 확인했습니다. `/app?x=1` URL을 유지하면서 `PHP:8.3.0`을 수집했고, Feroxbuster 인자에 `php`가 추가되는 것을 확인했습니다. `TestInstalledWebOnlyFingerprintOnLoopback`으로 다시 실행할 수 있습니다.
- Web-only에서 사용하는 `-nfs`도 저장된 실제 Windows 도움말과 대조합니다.

## Go 설치 지원 검증

- 모듈 경로와 내부 import를 `github.com/naroSEC/DiscoveryX`로 맞췄습니다.
- `go install ./cmd/discoveryx`를 별도 `GOBIN`에 실행한 뒤, 소스와 설정 파일이 없는 작업 폴더에서 `--version`, `--help`, `tools list`를 확인했습니다.
- 기본 YAML과 기술별 확장자 룰은 `go:embed`로 포함합니다. 소스 외부에서 기본 설정을 읽고, 내장 룰의 스냅샷 생성과 Web-only 실행·재개가 동작하는 테스트를 추가했습니다. 명시한 사용자 룰 파일이 없으면 내장 룰로 조용히 대체하지 않습니다.

## 실행하지 못한 추가 검사

`go test -race ./... -count=1`은 테스트 실행 전에 `runtime/cgo: ... cgo.exe: exit status 2`로 빌드 실패했습니다. 발견된 C 컴파일러는 `C:\msys64\mingw32\bin\gcc.exe`이며 Go 실행 대상은 windows/amd64입니다. 기본 테스트와 일반 Go 빌드는 통과했으나 race detector 통과로 표시하지 않습니다.

실제 Naabu SYN/Npcap 실행, 외부 서비스에서의 Subfinder/Dnsx 탐색, 실제 취약점 템플릿의 대상 스캔, 실제 Linux 환경 동작은 검증하지 않았습니다. 외부 대상 대신 mock과 루프백으로 핵심 흐름을 검증했습니다. Nuclei 템플릿 최초 설치/업데이트의 실제 네트워크 실행은 수행하지 않았으며 모의 실행 파일로 성공/실패와 롤백을 검증했습니다.
