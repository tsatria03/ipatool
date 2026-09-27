' Double-click this file to start the ipatool GUI straight from its Go source.
' It runs "go run" in this folder with no console window. Go must be installed.
' The first start compiles the GUI (about a minute); later starts take a second or two.
Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("WScript.Shell")
shell.CurrentDirectory = fso.GetParentFolderName(WScript.ScriptFullName)
shell.Run "go run -ldflags=-H=windowsgui .", 0, False
