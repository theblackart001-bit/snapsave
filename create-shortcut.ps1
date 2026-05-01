$ws = New-Object -ComObject WScript.Shell
$sc = $ws.CreateShortcut("$env:USERPROFILE\Desktop\SnapSave.lnk")
$sc.TargetPath = "$env:USERPROFILE\video-downloader\release\win-unpacked\SnapSave.exe"
$sc.WorkingDirectory = "$env:USERPROFILE\video-downloader\release\win-unpacked"
$sc.Description = "SnapSave Video Downloader"
$sc.Save()
