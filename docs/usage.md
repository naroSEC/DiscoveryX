# 사용 가이드

Go 기반 Windows 우선 보안 정보 수집 CLI입니다. 승인한 도메인과 하위 도메인을 대상으로 공식 스캐너를 순차 실행하고, 원본 출력·정규화 결과·상태·요약을 로컬에 저장합니다. 외부 스캐너의 동작을 재구현하지 않습니다.

## 빌드와 시작

Go 1.25 이상과 네트워크 연결이 필요합니다. 프로젝트 루트에서 실행하세요.

```powershell
go build -o discoveryx.exe ./cmd/discoveryx
.\discoveryx.exe --help
.\discoveryx.exe --version
.\discoveryx.exe tools install
.\discoveryx.exe tools list
.\discoveryx.exe doctor
```

필요한 도구가 없으면 해당 단계에서 공식 GitHub Releases 자산을 설치합니다. 기본 위치는 `%LOCALAPPDATA%\discoveryx\tools`이며 템플릿은 인접한 `templates` 디렉터리에서 관리합니다. Linux에서는 `os.UserCacheDir()` 아래를 사용합니다. `--tools-dir`, `--templates-dir`로 바꿀 수 있습니다. PATH의 동명 실행 파일을 임의로 선택하지 않으며, 정상적인 Manifest와 바이너리 해시가 있으면 재다운로드하지 않습니다.

디렉터리 탐색을 사용하려면 **사용자가 직접** `wordlists/common.txt`를 배치하세요. DiscoveryX는 사전을 생성하거나 다운로드하지 않습니다. 템플릿도 다음 명령으로 명시적으로 설치해야 합니다.

```powershell
.\discoveryx.exe templates update
.\discoveryx.exe templates version
```

## 실행 예제

아래 도메인은 사용법을 설명하기 위한 값입니다. 실제 실행 시 승인받은 대상으로 바꾸세요.

```powershell
.\discoveryx.exe -u example.com
.\discoveryx.exe -u example.com -ps last
.\discoveryx.exe -u example.com -ps both
.\discoveryx.exe -u example.com -ps off -cwl off
.\discoveryx.exe -u example.com -sbf off
.\discoveryx.exe -u https://example.com/app?q=1 --web-only
.\discoveryx.exe -u example.com --wordlist "C:\보안 자료\common.txt" --output "C:\진단 결과"
.\discoveryx.exe -u example.com --approved-ips 192.0.2.10 --ports 80,443,8000-8100
.\discoveryx.exe resume --run "runs\example.com_실행시각"
```

기본 순서는 Naabu → Subfinder → Dnsx → Httpx → Katana → Feroxbuster → Nuclei입니다. `-ps first/last/both/off`로 포트 단계 위치를 정하고 `-sbf`, `-cwl`, `--dir`, `--nuc`는 `on/off`를 받습니다. 긴 별칭은 `--port-scan`, `--subfinder`, `--crawl`입니다. Cobra에 전달하기 전에 정확한 `-ps`, `-sbf`, `-cwl` 토큰과 `=값` 형식을 변환하므로 PowerShell/CMD에서도 멀티 문자 단축 옵션을 사용할 수 있습니다. 옵션 뒤의 `--`는 변환하지 않습니다.

`-sbf off`라도 Dnsx와 Httpx는 원본 도메인으로 실행됩니다. 앞 단계가 실패해도 사용할 수 있는 입력이 있으면 다음 단계를 수행합니다. 웹 URL이 없으면 웹 단계는 사유를 남기고 건너뜁니다. 사전이나 템플릿이 없거나 템플릿 필터 결과가 0개인 경우도 성공적인 점검으로 표시하지 않습니다. 전체 성공은 종료 코드 0, 잘못된 설정·실패·부분 완료·중단은 1입니다. 요약의 각 단계 상태로 이유를 확인하세요.

`--web-only`가 Naabu/Subfinder/Dnsx/Httpx 설정보다 우선하며 세 웹 단계만 계획합니다. 세 웹 단계의 비활성화 옵션은 계속 적용합니다. 완전한 HTTP(S) URL은 경로와 쿼리를 유지해 사용합니다. 도메인만 주면 Go HTTP 클라이언트가 HTTPS, HTTP 순으로 HEAD 요청을 보내며 리다이렉트를 따라가지 않습니다. 하나의 입력 URL을 사용하고 크롤링 결과를 디렉터리/Nuclei의 새 대상으로 추가하지 않습니다.

## 승인 범위와 부하

호스트 경계로 비교하므로 `api.example.com`은 허용하고 `example.com.attacker.test`는 차단합니다. `--approved`는 추가로 승인한 도메인, `--exclude`는 제외할 도메인과 그 하위 호스트입니다. CNAME, CDN, 외부 링크는 승인 범위를 넓히지 않습니다. 외부 결과는 별도로 저장하며 다음 단계 입력을 만들 때 다시 검사합니다.

공유 CDN/호스팅 IP의 소유권은 DNS만으로 판별할 수 없습니다. **현재 버전은 포트 스캔 IP에 명시적인 `--approved-ips` 승인을 요구합니다.** 입력 자체가 IP이면 그 IP는 승인으로 취급합니다. 도메인에서 해석된 미승인 IP는 로그에 기록하고 스캔하지 않습니다. 승인 IP가 없으면 포트 단계는 Skipped가 됩니다. 이 정책은 단순히 도메인이 가리킨다는 이유로 제3자 공유 IP를 스캔하는 것을 막기 위한 보수적인 구현입니다.

포트 스캔은 도메인을 직접 넘기는 대신 승인된 IP를 고정해 전달하고, 결과에는 원래 호스트명을 연결합니다. `both`에서는 동일 실행 안에서 완료한 IP와 동일 조건의 포트 스캔을 재실행하지 않습니다. 마지막 단계는 유효한 호스트 중 새롭게 승인된 IP만 사용합니다. 포트 스캔 기본은 Connect(`-s c`)이며 Smart Scan(`-ss`)은 별도 옵션입니다. SYN(`--scan-type syn`)은 Windows 관리자 토큰과 Npcap 파일을 검사하며, `--syn-fallback`을 명시한 경우에만 경고 후 Connect로 대체합니다. 드라이버가 실제로 작동하는지는 Naabu 실행 결과로 최종 판단합니다.

Katana는 입력 FQDN의 정규식과 제외 범위, `-fs fqdn`, 리다이렉트 비활성화를 사용합니다. Httpx도 리다이렉트 추적 옵션을 켜지 않습니다. Feroxbuster는 대상별 순차 실행, `--scan-limit 1`, 제한된 스레드·속도·깊이를 사용하며 링크 추출을 끄고 사전 기반 재귀 탐색을 수행합니다. 디렉터리별 rate limit이 합산되지 않도록 동시 디렉터리 스캔을 하나로 제한합니다. 기존 Feroxbuster 전역/실행 파일 옆 설정이 있으면 범위와 리다이렉트 정책이 섞일 수 있어 해당 단계를 중단합니다.

하나의 DiscoveryX 실행에서 한 번에 하나의 스캐너 프로세스만 실행합니다. 단계별 전역 요청률이 호스트별 요청률의 상한도 제한합니다. 대상 목록 기본 상한은 100개입니다. 실행 디렉터리·도구 디렉터리·템플릿 관리에는 OS 파일 잠금을 사용하므로 동일 도구 경로의 스캔 중 업데이트를 막고, 프로세스가 종료되면 재시작 후에도 잠금이 자동 해제됩니다.

## 설정

우선순위는 **CLI > 환경 변수 > YAML > 기본값**입니다. 현재 작업 폴더의 `config.yaml`, 없으면 `configs/config.yaml`을 읽으며 `--config`로 파일을 지정할 수 있습니다. `configs/config.yaml`에 워크플로우, 속도, 동시성, 시간 제한, 사전, 범위, Nuclei와 출력/로그 기본 설정이 있습니다. 경로는 실행 시 작업 디렉터리 기준으로 해석한 뒤 절대 경로로 저장합니다.

환경 변수는 YAML 키에 `DISCOVERYX_`를 붙여 대문자로 만듭니다. 목록·맵은 YAML 형식을 받습니다.

```powershell
$env:DISCOVERYX_MAX_TARGETS = '50'
$env:DISCOVERYX_HTTP_TIMEOUT = '5s'
$env:DISCOVERYX_APPROVED = '[api.example.net]'
$env:DISCOVERYX_LIMITS = '{naabu: {rate: 10, concurrency: 2}}'
.\discoveryx.exe -u example.com --katana-rate 10 --katana-concurrency 2
```

`DISCOVERYX_LIMITS`는 맵 전체를 대체하므로 사용할 때 7개 도구 항목을 모두 지정해야 합니다. 일부 값만 바꾸려면 YAML 또는 도구별 CLI 옵션을 권장합니다. 0/음수 속도·시간·깊이, 누락된 도구 제한, 잘못된 범위와 포트는 스캔 전에 거부합니다. 동시성은 도구별 최대 100입니다.

주요 CLI 설정은 `--ports`, `--scan-type`, `--smart-scan`, `--service-version=false`, `--probes`, `--http-timeout`, `--stage-timeout`, `--max-duration`, `--max-targets`, `--crawl-depth`, `--dir-depth`, `--extensions`, `--nuclei-tags`, `--nuclei-severity`, `--log-level`입니다. 모든 도구에 `--<도구>-rate`, `--<도구>-concurrency`가 있습니다. Subfinder는 수동 수집 모드이므로 동시성 설정은 DNS 해석 작업을 하는 Dnsx에 적용되며 Subfinder 자체에는 요청률만 전달합니다.

기술별 확장자는 `configs/tech-extensions.yaml`에 대소문자 무시 정규식으로 관리합니다. 기본 `xml,txt,html`에 PHP→php, Java/JSP→jsp, ASP/ASP.NET→asp/aspx를 추가합니다. 중복과 대문자를 정리하며 IIS만으로 ASP.NET을 추정하지 않습니다. 기술 탐지는 휴리스틱입니다.

## 실행 파일과 템플릿 업데이트

```powershell
.\discoveryx.exe tools update
.\discoveryx.exe tools update --tool naabu
.\discoveryx.exe tools install --tool httpx
.\discoveryx.exe templates update
```

평상시 스캔과 `tools list`는 이미 설치된 도구의 최신 릴리스를 조회하지 않습니다. 누락된 도구의 최초 설치와 명시적인 `tools update`만 Releases API를 사용합니다. Draft/Prerelease는 제외하고 SemVer로 비교합니다. 현재 OS/아키텍처용 공식 자산이 없거나 후보가 모호하면 설치하지 않습니다. 소스 자동 빌드와 임의 플랫폼 대체는 하지 않습니다.

다운로드는 크기 제한, timeout, 제한된 재시도와 backoff, 403/429/Retry-After 처리를 적용합니다. 긴 rate limit은 무한 대기하지 않고 오류로 반환합니다. `GITHUB_TOKEN`은 API 인증에만 사용할 수 있고 토큰은 Manifest나 로그에 기록하지 않습니다. 릴리스 SHA256 digest 또는 체크섬 파일이 있으면 검사합니다. 검증 자료가 없으면 **unverified**로 명시합니다. 별도 서명 체인 검증은 구현하지 않았으며 SHA256 검증과 구분합니다.

ZIP/tar.gz 내부 경로의 절대 경로·상위 탈출·Windows 드라이브·역슬래시 경로를 거부하고, 필요한 일반 실행 파일만 제한된 크기로 읽습니다. 후보는 버전/도움말과 릴리스 태그를 확인한 뒤 해시별 새 경로에 설치하고 Manifest를 전환합니다. 다운로드·검증·Manifest 저장 실패 시 이전 실행 파일과 Manifest를 유지합니다. Windows에서 사용 중인 실행 파일을 덮어쓰지 않습니다. 이전 실행 파일과 템플릿 백업은 복구를 위해 보존하며 자동 정리하지 않습니다.

ProjectDiscovery 도구는 설치된 버전의 도움말에 `-duc`가 있어야 하며 실행 시 전달합니다. 자체 엔진 업데이트 인자와 Feroxbuster `-U/--update`는 사용하지 않습니다. 템플릿 업데이트는 **오직** `templates update`에서 `nuclei -ut -ud <임시 경로>`로 수행합니다. 기존 템플릿과 분리해 갱신하고 성공·트리 해시·실제 버전 출력을 기록한 뒤 관리 경로를 전환합니다. 실패하면 기존 트리를 유지합니다. `tools update`는 템플릿을 갱신하지 않습니다.

Nuclei 스캔은 설치된 템플릿을 로컬에서 필터링하고 복사본을 사용합니다. 태그별 설치 수·정책을 통과한 수·엔진 후보 목록 수와 실제로 로드한 서명 템플릿 수를 `inputs/template-filters.json`에 기록합니다. 고정된 BaseURL/RootURL 경로를 사용하는 서명된 HTTP GET/HEAD 템플릿만 허용합니다. raw 요청, payloads, DSL, flow, 외부 고정 URL, 코드, headless, OAST, fuzz/DoS 관련 태그는 거부합니다. `-dut`, `-pt http`, `-ni`, `-dr`, `-duc`를 적용하고 엔진 `-tl`로 후보 목록을 확인하고, 실행 로그에서 실제 로드한 서명 템플릿 수를 별도로 검증합니다. `-tl` 목록만으로 서명 유효성을 판단하지 않으며, 실제 로드 수가 0이거나 확인할 수 없으면 성공으로 표시하지 않습니다. 이 보수적인 정책은 전체 공식 CVE 템플릿보다 점검 범위를 줄입니다. 자동으로 공격적인 기능을 활성화하지 않습니다. 결과는 확정 취약점이 아닌 검토가 필요한 탐지 이벤트입니다.

## 결과와 재개

실행마다 `runs/<호스트>_<나노초 포함 시각>/`을 만듭니다.

- `metadata.json`: 설정, 범위, 도구 버전/해시, 리소스와 템플릿 식별 정보.
- `status.json`: Pending/Running/Completed/Skipped/Failed, 소요 시간, 레코드 수, 완료 출력의 SHA256.
- `inputs/`: 범위, 정제 호스트, `web-urls.txt`, 스캐너별 독립 설정, 사전/룰/템플릿 스냅샷.
- `raw/`: 스캐너 원본 JSONL(Subfinder는 TXT), 대상별 Feroxbuster JSONL, stdout/stderr 로그.
- `normalized/`: assets, ports, webservices, urls, vulnerabilities, invalid-hosts, out-of-scope JSON과 단계 체크포인트.
- `logs/`: execution.log, errors.log.
- `summary.json`: 개수, 실행 시간, 단계별 결과와 출력 디렉터리.

Feroxbuster는 여러 대상을 각각 `feroxbuster_0000.jsonl` 등의 파일에 보존합니다. 원본 이벤트는 삭제하지 않고 정규화 URL을 중복 제거합니다. Nuclei는 원본/정규화 이벤트를 모두 보존하면서 중복 이벤트에 `duplicate_of`를 연결합니다. 탐지 시간과 템플릿 버전/트리 해시, 원본 행 번호를 함께 저장합니다. 비정상 JSONL 행은 건너뛰면서 파싱 경고와 단계 실패를 기록하고 나머지 레코드를 보존합니다.

Ctrl+C는 context를 취소하며 Windows Job Object 또는 Unix 프로세스 그룹을 통해 자식 프로세스를 종료하고 상태와 결과를 저장합니다. Windows Job 할당이 불가능하면 격리 없이 계속하지 않습니다. 강제 전원 종료까지 저장을 보장할 수는 없으므로, 재개 시 상태뿐 아니라 체크포인트와 필수 원본 파일 해시를 검증합니다.

`resume --run <디렉터리>`는 저장된 설정을 사용합니다. CLI로 다른 대상이나 범위를 덮어쓰지 않습니다. 도구 버전/바이너리, 사용한 사전·룰, 템플릿 트리가 달라졌으면 재개를 거부합니다. 검증된 연속 완료 단계는 재사용하고 첫 실패·건너뜀·손상 단계부터 이후 단계를 재실행합니다. 재시도 전 원본 파일은 `.previous-<시각>` 이름으로 보존합니다. 실행 시간 상한은 재개 전후의 활성 시간을 합산합니다.

## 구조와 확장

`internal/cli`, `config`, `runner`, `tools`, `updater`, `engine`, `scope`, `model`, `parser`, `storage`로 책임을 분리했습니다. 도구는 `Name/Version/Validate/BuildArgs/Run/Parse/RequiredInputs/Outputs` 인터페이스를 구현합니다. `Registry()`에 어댑터를 등록하고 `engine.Plan()`에 단계 설명을 추가해 확장할 수 있습니다. 현재 입력 연결은 호스트/웹 서비스/포트/템플릿 유형을 중심으로 구현되어 있으며, GitHub/Cloud처럼 다른 유형은 입력 연결도 추가해야 합니다. 해당 기능, 대시보드, AI 기능, DB 서버는 구현하지 않았습니다.

## 검증과 현재 제약

```powershell
go test ./... -count=1
go vet ./...
go build -o discoveryx.exe ./cmd/discoveryx
# 선택: 이미 설치한 공식 바이너리를 루프백 서버에서만 검사
$env:DISCOVERYX_TEST_TOOLS = (Resolve-Path .validation/tools).Path
go test ./internal/tools -run TestInstalledScannersOnLoopback -v -count=1
Remove-Item Env:DISCOVERYX_TEST_TOOLS
```

기본 테스트는 외부 대상을 스캔하지 않습니다. 빌드한 모의 Go 실행 파일, 로컬 HTTP/GitHub API 서버로 옵션/실행 순서, 중간 전달, 확장자, JSONL 오류, 중복, 버전 비교, 재설치 방지, 업데이트 롤백/체크섬, 템플릿 정책과 롤백, Windows 프로세스 트리 종료와 한글 경로, 범위, 재개/무결성을 검사합니다. 실제 스캐너로 검사하는 선택 테스트도 루프백에만 요청하고 외부 호스트 링크에 요청이 없었는지 확인합니다.

공식 릴리스 도움말 원본과 확인된 차이는 [공식 릴리스 호환성](compatibility.md)에 있습니다. Naabu 2.6.1에는 `-sV`가 있지만 `-sV-probes`가 없어 현재 어댑터는 서비스 버전 탐지를 비활성화하고 일반 포트 스캔을 계속합니다. 향후 해당 옵션을 제공하는 바이너리가 설치되고 프로브 파일이 있을 때 활성화합니다. Dnsx 1.3.1에는 `-config`가 없어 전달하지 않습니다.

Linux는 컴파일을 확인할 수 있도록 OS별 프로세스/파일 잠금을 분리했으나 실제 Linux 스캐너와 권한/드라이버 동작 검증은 별도로 필요합니다. 도메인/범위는 ASCII 또는 punycode를 사용합니다. 대규모 출력은 JSONL을 스트리밍으로 읽지만 정규화 모델은 메모리에 유지하므로 큰 실행은 `--max-targets`, 깊이와 시간 제한을 낮추세요. 완전한 OS 네트워크 샌드박스를 제공하지 않으며 스캐너 자체의 범위 구현을 함께 사용합니다. 사용 중인 템플릿/룰/실행 파일은 같은 사용자에 의해 변경되지 않도록 관리하세요.
