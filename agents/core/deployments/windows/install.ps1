param(
    [string]$InstallDir = "C:\Program Files\RMM Agent",
    [string]$ServerUrl = "https://rmm.example.com",
    [string]$TenantId = "",
    [string]$AgentId = ""
)

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Path ".\rmm-agent.exe" -Destination "$InstallDir\rmm-agent.exe" -Force

[Environment]::SetEnvironmentVariable("RMM_SERVER_URL", $ServerUrl, "Machine")
[Environment]::SetEnvironmentVariable("RMM_TENANT_ID", $TenantId, "Machine")
[Environment]::SetEnvironmentVariable("RMM_AGENT_ID", $AgentId, "Machine")

sc.exe create RMMAgent binPath= "`"$InstallDir\rmm-agent.exe`"" start= auto
sc.exe start RMMAgent
