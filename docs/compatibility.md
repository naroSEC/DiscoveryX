# 공식 문서 및 Windows 바이너리 호환성

2026-10-20에 공식 GitHub Releases API로 Windows/amd64 안정 릴리스를 조회해 다운로드했습니다. 배포 자산의 SHA256을 검사하고 실제 `-h`/`--help`, `-version -duc`/`--version` 실행으로 확인했습니다. 원본 도움말은 이 디렉터리의 `<도구>-help.txt`입니다. 바이너리와 Manifest는 검증용 `.validation/tools/`에 보존되며 기본 사용자 설치 경로와 분리되어 있습니다.

- Naabu 2.6.1 — [공식 릴리스](https://github.com/projectdiscovery/naabu/releases/tag/v2.6.1), [문서](https://github.com/projectdiscovery/naabu).
- Subfinder 2.17.0 — [공식 릴리스](https://github.com/projectdiscovery/subfinder/releases/tag/v2.17.0), [문서](https://github.com/projectdiscovery/subfinder).
- Dnsx 1.3.1 — [공식 릴리스](https://github.com/projectdiscovery/dnsx/releases/tag/v1.3.1), [문서](https://github.com/projectdiscovery/dnsx).
- Httpx 1.12.0 — [공식 릴리스](https://github.com/projectdiscovery/httpx/releases/tag/v1.12.0), [문서](https://github.com/projectdiscovery/httpx).
- Katana 1.8.0 — [공식 릴리스](https://github.com/projectdiscovery/katana/releases/tag/v1.8.0), [문서](https://github.com/projectdiscovery/katana).
- Feroxbuster 2.13.1 — [공식 릴리스](https://github.com/epi052/feroxbuster/releases/tag/v2.13.1), [옵션 소스](https://github.com/epi052/feroxbuster/blob/v2.13.1/src/parser.rs).
- Nuclei 3.11.1 — [공식 릴리스](https://github.com/projectdiscovery/nuclei/releases/tag/v3.11.1), [문서](https://github.com/projectdiscovery/nuclei).

최신 저장소 소스와 실제 릴리스의 차이는 다음과 같이 처리합니다.

1. Naabu의 현재 README에는 `-sV-probes`가 있지만 실제 2.6.1 도움말에는 없습니다. 미지원 인자를 전달하지 않으며, 프로브 파일이 있어도 이 버전에서는 서비스 버전 탐지를 경고와 함께 비활성화합니다. 일반 포트 스캔은 계속합니다. `-ss`는 실제 도움말에서도 Smart Scan입니다.
2. Dnsx 1.3.1에는 `-config`가 없습니다. 이 어댑터는 해당 옵션을 전달하지 않고 `-duc`와 명시적인 스캔 설정을 사용합니다.
3. Feroxbuster의 Windows 릴리스에는 debug 자산도 있으므로 CPU에 맞는 일반 자산만 선택합니다. `--json`은 로그가 포함된 JSONL을 출력하므로 정규화에서는 response 이벤트만 채택합니다.
4. Feroxbuster의 설정 병합은 기본값 false로 이전의 true를 취소하지 못하는 경우가 있습니다. 전역 설정이나 실행 파일 옆 설정을 발견하면 해당 단계를 중단하고, 독립된 작업 디렉터리의 설정을 사용합니다. 링크 추출은 비활성화하고 대상을 순차 실행합니다.
5. Nuclei 템플릿은 일반 스캔에서 설치하지 않습니다. 로컬 필터 뒤 실제 `-tl` 결과도 확인하고, 0개이면 점검한 것으로 취급하지 않습니다. `-ut`는 명시적인 templates update 명령에서만 사용합니다.
6. 실제 Nuclei 3.11.1의 `-tl -dut`는 서명되지 않은 로컬 파일도 목록에 표시할 수 있습니다. 따라서 목록 수를 유효한 서명 검증 결과로 간주하지 않습니다. 로컬 필터에서 digest 표시가 없는 파일을 제외하고, 실제 스캔의 `-dut`와 엔진 로그의 로드된 템플릿 수를 별도로 확인합니다. 로드된 수가 0이거나 확인할 수 없으면 성공 처리하지 않습니다. digest 표시 검사 자체는 암호학적 서명 검증이 아니며, 서명 검증은 Nuclei 엔진이 담당합니다.

`TestArgumentsAgainstVerifiedWindowsHelp`는 이 실제 도움말과 7개 어댑터의 인자를 대조합니다. 향후 설치된 버전에서도 각 실행 전에 해당 바이너리의 도움말을 다시 검사합니다. 필요한 기능이 없으면 임의의 다른 인자로 대체하지 않고 해당 단계를 실패 처리합니다. 위에서 설명한 선택적 서비스 버전 탐지는 경고 후 일반 스캔을 계속합니다.
