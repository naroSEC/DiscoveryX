# Directory wordlist

`common.txt`는 사용자가 제공한 Feroxbuster 기본 사전입니다. 파일 내용은 수정하지 않고 실행 파일에 내장합니다.

별도의 사전을 지정하지 않으면 내장된 파일을 사용합니다. `go install`로 설치하거나 소스 저장소 밖에서 실행해도 사전을 따로 복사할 필요가 없습니다. 기본 설정은 `wordlist: builtin`이며 `--wordlist builtin`으로 직접 선택할 수도 있습니다.

다른 사전은 `--wordlist <경로>`로 지정하세요. CLI > 환경 변수 > 설정 파일 > 기본값 순서로 적용합니다. 지정한 파일이 없으면 디렉터리 단계를 건너뛰고 원인을 기록하며, 내장 사전으로 조용히 대체하지 않습니다.

Feroxbuster는 실제 파일 경로가 필요하므로 실행 전에 선택한 사전을 `inputs/wordlist.txt`에 저장합니다. DiscoveryX는 외부 사전을 자동 다운로드하지 않습니다.
