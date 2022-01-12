set GOARCH=amd64
set GOOS=linux
go build
mkdir turboclip_bot
xcopy turboclip turboclip_bot /Y
xcopy config.toml turboclip_bot /Y
xcopy messages.toml turboclip_bot /Y