# 사용 가이드

Go 기반 Windows 우선 보안 정보 수집 CLI입니다. 입력한 도메인과 하위 도메인을 대상으로 공식 스캐너를 순차 실행하고, 원본 출력·정규화 결과·상태·요약을 로컬에 저장합니다. 외부 스캐너의 동작을 재구현하지 않습니다.

## 빌드와 시작

Go 1.25 이상과 네트워크 연결이 필요합니다. 저장소를 내려받지 않고 설치하려면 다음 명령을 사용하세요.

```powershell
go install github.com/naroSEC/DiscoveryX/cmd/discoveryx@latest
discoveryx --help
```

실행 파일은 `go env GOBIN`의 경로에 설치됩니다. 해당 값이 비어 있으면 `$(go env GOPATH)\bin`을 사용하므로 그 폴더를 PATH에 추가해 주세요. 설치된 명령은 아래 예제의 `.\discoveryx.exe` 대신 `discoveryx`로 실행할 수 있습니다.

기본 설정과 기술별 확장자 룰, 기본 사전은 실행 파일에 포함됩니다. 현재 폴더의 `config.yaml` 또는 `configs/config.yaml`이 있으면 해당 파일을 읽으며, 없으면 내장 기본 설정을 사용합니다. 기본 기술 룰 파일이 없으면 내장 룰로 동작하고, 별도의 `--tech-rules <경로>`를 지정하면 그 파일을 사용합니다. `--tech-rules builtin`으로 내장 룰을 직접 선택할 수도 있습니다. 사전은 기본적으로 `wordlist: builtin`으로 선택합니다. 다른 사전은 `--wordlist <경로>`로 지정할 수 있으며, 없는 사용자 파일을 내장 사전으로 조용히 대체하지 않습니다.

소스에서 직접 빌드하려면 프로젝트 루트에서 다음을 실행하세요.

```powershell
go build -o discoveryx.exe ./cmd/discoveryx
.\discoveryx.exe --help
.\discoveryx.exe --version
.\discoveryx.exe tools install
.\discoveryx.exe tools list
.\discoveryx.exe doctor
```

필요한 도구가 없으면 해당 단계에서 공식 GitHub Releases 자산을 설치합니다. 기본 위치는 `%LOCALAPPDATA%\discoveryx\tools`이며 템플릿은 인접한 `templates` 디렉터리에서 관리합니다. Linux에서는 `os.UserCacheDir()` 아래를 사용합니다. `--tools-dir`, `--templates-dir`로 바꿀 수 있습니다. PATH의 동명 실행 파일을 임의로 선택하지 않으며, 정상적인 Manifest와 바이너리 해시가 있으면 재다운로드하지 않습니다.

별도 사전을 지정하지 않으면 내장된 `common.txt`를 사용합니다. Feroxbuster 실행 전에 해당 내용을 실행 폴더의 `inputs/wordlist.txt`에 저장하고 실제 파일 경로로 전달합니다. 사용자 사전도 같은 방식으로 스냅샷을 보존합니다. 작업 폴더나 소스 저장소가 없어도 기본 사전을 사용할 수 있으며, 외부 사전을 다운로드하지 않습니다. Nuclei 템플릿은 없거나 YAML 파일이 하나도 없으면 최초 점검 전에 자동 설치합니다. 기존 템플릿이 있으면 다운로드나 업데이트를 하지 않습니다. 미리 설치하거나 갱신하려면 다음 명령을 사용하세요.

```powershell
.\discoveryx.exe templates update
.\discoveryx.exe templates version
```

## 실행 예제

아래 도메인은 사용법을 설명하기 위한 값입니다. 실행할 대상 도메인으로 바꿔 주세요.

```powershell
.\discoveryx.exe -u example.com
.\discoveryx.exe -u example.com -ps last
.\discoveryx.exe -u example.com -ps both
.\discoveryx.exe -u example.com -ps off -cwl off
.\discoveryx.exe -u example.com -sbf off
.\discoveryx.exe -u https://example.com/app?q=1 --web-only
.\discoveryx.exe -u example.com --wordlist "C:\보안 자료\common.txt" --output "C:\진단 결과"
.\discoveryx.exe -u example.com --ports 80,443,8000-8100
.\discoveryx.exe resume --run "runs\example.com_실행시각"
```

기본 순서는 Naabu → Subfinder → Dnsx → Httpx → Katana → Feroxbuster → Nuclei입니다. `-ps first/last/both/off`로 포트 단계 위치를 정하고 `-sbf`, `-cwl`, `--dir`, `--nuc`는 `on/off`를 받습니다. 긴 별칭은 `--port-scan`, `--subfinder`, `--crawl`입니다. Cobra에 전달하기 전에 정확한 `-ps`, `-sbf`, `-cwl` 토큰과 `=값` 형식을 변환하므로 PowerShell/CMD에서도 멀티 문자 단축 옵션을 사용할 수 있습니다. 옵션 뒤의 `--`는 변환하지 않습니다.

`-sbf off`라도 Dnsx와 Httpx는 원본 도메인으로 실행됩니다. 앞 단계가 실패해도 사용할 수 있는 입력이 있으면 다음 단계를 수행합니다. 웹 URL이 없으면 웹 단계는 사유를 남기고 건너뜁니다. 직접 지정한 사전이나 템플릿이 없거나 템플릿 필터 결과가 0개인 경우도 성공적인 점검으로 표시하지 않습니다. 전체 성공은 종료 코드 0, 잘못된 설정·실패·부분 완료·중단은 1입니다. 요약의 각 단계 상태로 이유를 확인하세요.

`--web-only`에서는 Naabu/Subfinder/Dnsx를 건너뛰고 Httpx → Katana → Feroxbuster → Nuclei 순서로 진행합니다. Httpx는 단일 입력 URL의 상태·제목·기술 스택을 수집하며 디렉터리 확장자 선택에 전달합니다. `-nfs`로 입력 스킴을 유지하고 리다이렉트를 따라가지 않습니다. 세 웹 단계의 비활성화 옵션도 적용합니다. 완전한 URL은 스킴·포트·경로·쿼리를 유지하며, 도메인만 주면 Go HTTP 클라이언트가 HTTPS와 HTTP 순서로 HEAD 요청을 확인합니다. Httpx 실패 시에도 원래 입력 URL로 다음 단계를 진행합니다.

## GHRecon 병행 실행

기본 `ghrecon: auto`는 토큰이 있을 때만 GitHub 수집 단계를 활성화합니다. `--ghrecon on/off`로 명시할 수 있습니다. 토큰 입력은 `--github-token <값>` 또는 `--github-token-file <경로>` 중 하나를 사용하며, 없으면 `DISCOVERYX_GITHUB_TOKEN`, `GITHUB_TOKEN` 순서로 읽습니다. 토큰 파일에는 토큰 한 개를 적습니다. 토큰은 YAML이나 저장된 실행 설정에 포함하지 않습니다.

```powershell
.\discoveryx.exe -u https://www.example.com --github-token-file "C:\보안 설정\github.token"
.\discoveryx.exe -u https://www.example.com --web-only --github-token "github_pat_..."
.\discoveryx.exe -u example.com --ghrecon off
```

직접 입력한 토큰은 사용자의 쉘 이력에는 남을 수 있습니다. DiscoveryX는 GHRecon 실행 인자에 토큰을 넣지 않고 자식 프로세스의 `GITHUB_TOKEN`으로 전달합니다. 다른 스캐너에는 GitHub 토큰을 전달하지 않습니다. 기록되는 최초 CLI 명령은 토큰을 마스킹하고, GHRecon의 JSONL·stderr도 실행 중 사용하는 인증 토큰을 마스킹합니다.

루트 도메인은 Public Suffix List 기준으로 추출합니다. `www.example.com`은 `example.com`, `api.example.co.kr`은 `example.co.kr`, `api.team.github.io`는 `team.github.io`입니다. IP·단일 라벨 호스트·공개 접미사 자체는 GHRecon 대상으로 사용하지 않습니다. 이 변환은 GitHub 검색어에만 적용되며 기존 스캐너의 탐색 범위를 변경하지 않습니다.

Httpx가 끝나면 GHRecon을 시작하고 기존 Katana → Feroxbuster → Nuclei 흐름을 이어갑니다. 크롤링을 끄더라도 GHRecon은 뒤의 웹 점검과 병행할 수 있습니다. 웹 단계가 모두 끝나면 GHRecon도 종료될 때까지 기다립니다. 각 작업은 단계 시간 제한과 전체 실행 취소를 공유하며, Ctrl+C에서는 두 작업을 종료하고 상태를 보존합니다.

GHRecon 기본 설정은 `ghrecon_profile: standard`, `ghrecon_max_queries: 50`, `ghrecon_max_results: 100`, `ghrecon_concurrency: 4`입니다. 각각 `--ghrecon-profile`, `--ghrecon-max-queries`, `--ghrecon-max-results`, `--ghrecon-concurrency`로 덮어쓸 수 있습니다. API 요청 Timeout은 `--http-timeout`을 사용합니다. GitHub rate limit에 따른 대기와 재시도는 GHRecon이 관리하며 전체·단계 실행 시간 제한이 적용됩니다.

GHRecon의 `--verbose` 로그는 별도 stderr 파일에 보관해 부분 실패 원인을 확인할 수 있습니다. 원본은 `raw/ghrecon.jsonl`, 정규화 결과는 `normalized/github-findings.json`, 독립 실행 상태는 `logs/ghrecon-state.json`에 저장합니다. HTML의 GitHub 수집 화면에서 저장소·도메인·URL·클라우드 자산 등 유형으로 필터링하고 출처 저장소·파일·검색어·원본 링크를 확인합니다. 결과는 기존 웹 점검 입력에 자동 합치지 않습니다.

GHRecon 종료 코드 2는 부분 완료로 표시하고 성공적으로 받은 결과를 보존합니다. 웹 점검은 계속 실행합니다. 재개 시 완료된 GitHub 작업은 원본과 독립 체크포인트의 해시를 검증해 재사용하므로 웹 단계가 재시도되어도 반복하지 않습니다. GHRecon을 다시 실행해야 한다면 토큰 파일·CLI 옵션·환경 변수 중 하나로 토큰을 다시 전달하세요.

```powershell
.\discoveryx.exe resume --run "runs\기존실행폴더" --github-token-file "C:\보안 설정\github.token"
.\discoveryx.exe tools install --tool ghrecon
.\discoveryx.exe tools update --tool ghrecon
```

GHRecon은 공식 GitHub Releases 바이너리로 설치하고, 일반 스캔 중 엔진 자체 업데이트를 실행하지 않습니다. JSONL은 스키마 v1을 파싱하고 알 수 없는 부가 필드는 무시합니다. 지원하지 않는 스키마와 비정상 행은 경고를 남기며, 완전한 성공으로 표시하지 않습니다.

## 탐색 범위와 부하

호스트 경계로 비교하므로 `api.example.com`은 포함하고 `example.com.attacker.test`는 제외합니다. `--exclude`로 특정 도메인과 그 하위 호스트를 탐색에서 뺄 수 있습니다. CNAME, CDN, 외부 링크가 발견되어도 입력한 대상 범위를 넓히지 않습니다. 외부 결과는 별도로 저장하고 다음 단계 입력을 만들 때 다시 검사합니다.

포트 스캔은 도메인을 직접 넘기는 대신 해석된 IP를 고정해 전달하고, 결과에는 원래 호스트명을 연결합니다. `both`에서는 동일 실행 안에서 완료한 IP와 동일 조건의 포트 스캔을 재실행하지 않습니다. 마지막 단계는 유효한 호스트 중 새롭게 발견한 IP만 사용합니다. 포트 스캔 기본은 Connect(`-s c`)이며 Smart Scan(`-ss`)은 별도 옵션입니다. SYN(`--scan-type syn`)은 Windows 관리자 토큰과 Npcap 파일을 검사하며, `--syn-fallback`을 명시한 경우에만 경고 후 Connect로 대체합니다. 드라이버가 실제로 작동하는지는 Naabu 실행 결과로 최종 판단합니다.

Katana는 입력 FQDN의 정규식과 제외 범위, `-fs fqdn`, 리다이렉트 비활성화를 사용합니다. Httpx도 리다이렉트 추적 옵션을 켜지 않습니다. Feroxbuster는 대상별 순차 실행과 깊이 제한을 유지하고, 링크 추출을 끄고 사전 기반 재귀 탐색을 수행합니다. 요청률·스레드·동시 디렉터리 수는 설치된 도구의 기본값을 사용합니다. 필요하면 `--feroxbuster-rate`, `--feroxbuster-concurrency`, `--feroxbuster-scan-limit`로 지정할 수 있습니다. 기존 Feroxbuster 전역/실행 파일 옆 설정이 있으면 범위와 리다이렉트 정책이 섞일 수 있어 해당 단계를 중단합니다.

기존 스캐너는 한 번에 하나씩 실행하며 GHRecon만 Httpx 이후 별도 작업으로 병행합니다. 동시에 실행되는 스캐너는 GHRecon과 기존 스캐너 각 하나씩, 최대 두 개입니다. 도구 설치는 Manifest 충돌을 방지하도록 직렬화합니다. 요청률과 동시성은 도구별 설정을 사용합니다. Feroxbuster의 rate limit은 디렉터리별로 적용되므로 동시 디렉터리 수에 따라 전체 요청률이 달라집니다. 대상 목록 기본 상한은 100개입니다. 실행 디렉터리·도구 디렉터리·템플릿 관리에는 OS 파일 잠금을 사용하므로 동일 도구 경로의 스캔 중 업데이트를 막고, 프로세스가 종료되면 재시작 후에도 잠금이 자동 해제됩니다.

## 설정

우선순위는 **CLI > 환경 변수 > YAML > 기본값**입니다. 현재 작업 폴더의 `config.yaml`, 없으면 `configs/config.yaml`을 읽으며 `--config`로 파일을 지정할 수 있습니다. `configs/config.yaml`에 워크플로우, 속도, 동시성, 시간 제한, 사전, 범위, Nuclei와 출력/로그 기본 설정이 있습니다. 경로는 실행 시 작업 디렉터리 기준으로 해석한 뒤 절대 경로로 저장합니다.

환경 변수는 YAML 키에 `DISCOVERYX_`를 붙여 대문자로 만듭니다. 목록·맵은 YAML 형식을 받습니다.

```powershell
$env:DISCOVERYX_MAX_TARGETS = '50'
$env:DISCOVERYX_HTTP_TIMEOUT = '5s'
$env:DISCOVERYX_LIMITS = '{naabu: {rate: 10, concurrency: 2}}'
.\discoveryx.exe -u example.com --katana-rate 10 --katana-concurrency 2
```

`DISCOVERYX_LIMITS`는 맵 전체를 대체하므로 사용할 때 7개 도구 항목을 모두 지정해야 합니다. 일부 값만 바꾸려면 YAML 또는 도구별 CLI 옵션을 권장합니다. 음수 속도, 0/음수 시간·깊이, 누락된 도구 제한, 잘못된 범위와 포트는 스캔 전에 거부합니다. Feroxbuster·Nuclei의 rate/concurrency는 0이면 해당 인자를 생략해 도구 기본값을 사용하며, 양수면 그대로 전달합니다. 다른 도구는 양수 속도와 1–100의 동시성을 사용합니다.

주요 CLI 설정은 `--ports`, `--scan-type`, `--smart-scan`, `--service-version=false`, `--probes`, `--http-timeout`, `--stage-timeout`, `--max-duration`, `--max-targets`, `--crawl-depth`, `--dir-depth`, `--extensions`, `--nuclei-tags`, `--nuclei-severity`, `--log-level`입니다. 모든 도구에 `--<도구>-rate`, `--<도구>-concurrency`가 있습니다. Subfinder는 수동 수집 모드이므로 동시성 설정은 DNS 해석 작업을 하는 Dnsx에 적용되며 Subfinder 자체에는 요청률만 전달합니다.

Feroxbuster·Nuclei의 기본 rate/concurrency와 `feroxbuster_scan_limit`은 0입니다. 이는 무조건 무제한 인자를 전달한다는 뜻이 아니라 해당 옵션을 생략한다는 뜻입니다. Nuclei에 강제하던 `-bs 1`도 제거했습니다. 확인한 Windows 릴리스 기준 Feroxbuster 2.13.1은 스레드 50, 요청률·동시 디렉터리 상한 없음이고, Nuclei 3.11.1은 초당 요청 150·동시 템플릿 25·템플릿별 동시 호스트 25입니다. 실제 기본값은 설치된 버전의 도움말을 기준으로 합니다.

기존 YAML에 양수 제한이 남아 있으면 그 값이 계속 우선합니다. CLI에서 `--feroxbuster-rate 0 --feroxbuster-concurrency 0 --feroxbuster-scan-limit 0 --nuclei-rate 0 --nuclei-concurrency 0`을 지정하면 도구 기본값으로 되돌릴 수 있습니다. 단계당 30분·전체 2시간의 시간 제한은 별개이며 `--stage-timeout`, `--max-duration`으로 늘릴 수 있습니다. 재개는 저장된 설정을 유지하므로 변경한 속도·동시성 설정을 적용하려면 새 실행을 시작하세요.

기술별 확장자는 `configs/tech-extensions.yaml`에 대소문자 무시 정규식으로 관리합니다. 기본 `xml,txt,html`에 PHP→php, Java/JSP→jsp, ASP/ASP.NET→asp/aspx를 추가합니다. 중복과 대문자를 정리하며 IIS만으로 ASP.NET을 추정하지 않습니다. 기술 탐지는 휴리스틱입니다.

이전 설정의 `wordlist: wordlists/common.txt`는 외부 파일 지정으로 처리됩니다. 내장 사전을 쓰려면 `wordlist: builtin`으로 바꾸거나 `--wordlist builtin`으로 덮어쓰세요. 명시적인 CLI 옵션은 환경 변수와 설정 파일보다 우선합니다.

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

ProjectDiscovery 도구는 설치된 버전의 도움말에 `-duc`가 있어야 하며 실행 시 전달합니다. 자체 엔진 업데이트 인자와 Feroxbuster `-U/--update`는 사용하지 않습니다. 기존 템플릿의 업데이트는 `templates update`에서 `nuclei -ut -ud <임시 경로>`로 수행합니다. 일반 스캔에서는 `template_auto_install: true`인 경우에만 비어 있거나 없는 관리 폴더에 최초 설치합니다. `--template-auto-install=false`로 끌 수 있으며, 이미 YAML 템플릿이 있으면 재설치하거나 갱신하지 않습니다. 최초 설치 실패는 단계 실패로 기록하고 `discoveryx templates update` 재시도를 안내합니다. 기존 템플릿과 분리해 갱신하고 성공·트리 해시·실제 버전 출력을 기록한 뒤 관리 경로를 전환합니다. 실패하면 기존 트리를 유지합니다. `tools update`는 템플릿을 갱신하지 않습니다.

Nuclei 스캔은 설치된 템플릿을 로컬에서 필터링하고 복사본을 사용합니다. 태그별 설치 수·정책을 통과한 수·엔진 후보 목록 수와 실제로 로드한 서명 템플릿 수를 `inputs/template-filters.json`에 기록합니다. 고정된 BaseURL/RootURL 경로를 사용하는 서명된 HTTP GET/HEAD 템플릿만 허용합니다. raw 요청, payloads, DSL, flow, 외부 고정 URL, 코드, headless, OAST, fuzz/DoS 관련 태그는 거부합니다. `-dut`, `-pt http`, `-ni`, `-dr`, `-duc`를 적용하고 엔진 `-tl`로 후보 목록을 확인하고, 실행 로그에서 실제 로드한 서명 템플릿 수를 별도로 검증합니다. `-tl` 목록만으로 서명 유효성을 판단하지 않으며, 실제 로드 수가 0이거나 확인할 수 없으면 성공으로 표시하지 않습니다. 이 보수적인 정책은 전체 공식 CVE 템플릿보다 점검 범위를 줄입니다. 자동으로 공격적인 기능을 활성화하지 않습니다. 결과는 확정 취약점이 아닌 검토가 필요한 탐지 이벤트입니다.

## 결과와 재개

실행마다 `runs/<호스트>_<나노초 포함 시각>/`을 만듭니다.

- `metadata.json`: 설정, 범위, 도구 버전/해시, 리소스와 템플릿 식별 정보.
- `status.json`: Pending/Running/Completed/Skipped/Failed, 소요 시간, 레코드 수, 완료 출력의 SHA256.
- `inputs/`: 범위, 정제 호스트, `web-urls.txt`, 스캐너별 독립 설정, 사전/룰/템플릿 스냅샷.
- `raw/`: 스캐너 원본 JSONL(Subfinder는 TXT), 대상별 Feroxbuster JSONL, stdout/stderr 로그.
- `normalized/`: github-findings, assets, ports, webservices, urls, vulnerabilities, invalid-hosts, out-of-scope JSON과 단계 체크포인트.
- `logs/`: execution.log, errors.log.
- `summary.json`: 개수, 실행 시간, 단계별 결과와 출력 디렉터리.
- `report.html`: 데이터와 화면 리소스를 포함한 단일 HTML 리포트.
- `logs/commands.json`: 스캔 단계에서 실제 호출한 실행 파일과 인자, 시작 시각·소요 시간·상태·로그 경로.

### HTML 리포트

`report.html`을 브라우저에서 열면 개요와 자산·포트·웹 서비스·URL·디렉터리·탐지 결과·실행 단계·명령어를 구분해 볼 수 있습니다. 외부 링크와 미확인 호스트도 별도 화면에서 확인합니다. 화면 리소스는 파일에 포함되어 있어 CDN이나 별도 서버가 필요하지 않습니다.

검색과 호스트 필터, 탐지 결과의 심각도·중복 필터, 포트 스캔 시점 필터를 지원합니다. 표 머리글로 정렬하고 페이지당 표시 건수를 바꿀 수 있습니다. 레코드의 상세 버튼은 원본 JSON과 근거 경로를 보여 줍니다. JSON 내보내기는 현재 화면에서 필터링한 전체 레코드를 저장합니다. 개요 화면에서는 리포트 전체 데이터를 내보냅니다.

명령어 화면은 스캐너 실행과 사전 점검·템플릿 관리를 구분합니다. 실제 실행 파일 경로와 인자를 기록하고, 표시할 때 Windows에서는 PowerShell 형식으로 인용합니다. 복사한 명령을 자동 실행하지는 않습니다. 실행 상태와 소요 시간, 도구 버전, stdout/stderr 로그도 확인할 수 있습니다. 기록은 `status.json`에도 보존되어 재개 시 기존 이력에 이어집니다.

기존 결과로 화면만 다시 생성하려면 다음을 실행하세요. 스캐너 설치·실행이나 네트워크 요청은 하지 않습니다.

```powershell
.\discoveryx.exe report --run "runs\example.com_기존실행시각"
```

실행 중인 폴더에는 잠금을 적용합니다. 이전 버전에 명령어 이력이 없으면 미기록으로 표시하며, 결과만 보고 명령어를 재구성하지 않습니다. 리포트에는 수집 데이터와 로컬 실행 경로가 포함됩니다. 원본 로그 링크까지 유지하려면 실행 폴더 전체를 함께 보관하세요.

### 원본 보존과 재개


Feroxbuster는 여러 대상을 각각 `feroxbuster_0000.jsonl` 등의 파일에 보존합니다. 원본 이벤트는 삭제하지 않고 정규화 URL을 중복 제거합니다. Nuclei는 원본/정규화 이벤트를 모두 보존하면서 중복 이벤트에 `duplicate_of`를 연결합니다. 탐지 시간과 템플릿 버전/트리 해시, 원본 행 번호를 함께 저장합니다. 비정상 JSONL 행은 건너뛰면서 파싱 경고와 단계 실패를 기록하고 나머지 레코드를 보존합니다.

Ctrl+C는 context를 취소하며 Windows Job Object 또는 Unix 프로세스 그룹을 통해 자식 프로세스를 종료하고 상태와 결과를 저장합니다. Windows Job 할당이 불가능하면 격리 없이 계속하지 않습니다. 강제 전원 종료까지 저장을 보장할 수는 없으므로, 재개 시 상태뿐 아니라 체크포인트와 필수 원본 파일 해시를 검증합니다.

`resume --run <디렉터리>`는 저장된 설정을 사용합니다. CLI로 다른 대상이나 범위를 덮어쓰지 않습니다. 도구 버전/바이너리, 사용한 사전·룰, 템플릿 트리가 달라졌으면 재개를 거부합니다. 검증된 연속 완료 단계는 재사용하고 첫 실패·건너뜀·손상 단계부터 이후 단계를 재실행합니다. 재시도 전 원본 파일은 `.previous-<시각>` 이름으로 보존합니다. 실행 시간 상한은 재개 전후의 활성 시간을 합산합니다. 이전 버전의 Web-only 실행은 단계 구성이 달라 새 실행이 필요합니다. 기존 결과는 그대로 보존됩니다.

## 구조와 확장

`internal/cli`, `config`, `runner`, `tools`, `updater`, `engine`, `scope`, `model`, `parser`, `storage`, `reporting`으로 책임을 분리했습니다. 도구는 `Name/Version/Validate/BuildArgs/Run/Parse/RequiredInputs/Outputs` 인터페이스를 구현합니다. `Registry()`에 어댑터를 등록하고 `engine.Plan()`에 단계 설명을 추가해 확장할 수 있습니다. 현재 입력 연결은 호스트/웹 서비스/포트/템플릿 유형을 중심으로 구현되어 있으며, 다른 종류의 신규 데이터는 입력 연결도 추가해야 합니다. GHRecon의 별도 GitHub 수집은 이 구조에 병행 작업으로 연결했습니다. Cloud Discovery 모듈과 웹 서버 기반 대시보드, AI 기능, DB 서버는 구현하지 않았습니다.

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
