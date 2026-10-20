# Directory wordlist

`common.txt`는 사용자가 제공한 Feroxbuster 기본 사전입니다. 제공받은 파일의 내용을 수정하지 않고 포함했습니다.

프로젝트 루트에서 실행하면 기본 설정의 `wordlists/common.txt`를 사용합니다. 다른 사전은 `--wordlist <경로>`로 지정하세요.

`go install`로 설치한 실행 파일에는 사전이 내장되지 않습니다. 다른 작업 폴더에서 실행할 때는 이 파일의 경로를 `--wordlist`로 지정하거나 작업 폴더의 `wordlists/common.txt`에 배치하세요.

DiscoveryX는 외부 사전을 자동 다운로드하지 않습니다. 지정한 파일이 없으면 디렉터리 단계를 건너뛰고 원인을 기록합니다.
