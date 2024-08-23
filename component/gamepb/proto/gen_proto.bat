protoc --go_opt=paths=source_relative  --go_out=..   --proto_path=. *.proto
if %errorlevel% == 0 ( echo successfully ) else ( echo failed)