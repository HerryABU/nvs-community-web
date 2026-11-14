// logarchive 日志归档 CLI：按类型/日期打包（天 7z、周 7z、月 tar.gz）。
// 用法：go run ./server/cmd/logarchive [-log log] [-out log/archive]
package main

import (
	"flag"
	"log"

	"nvs-server/logarchive"
)

func main() {
	logDir := flag.String("log", "log", "日志根目录（含按类型/日期分层）")
	outDir := flag.String("out", "", "归档输出目录，默认 <log>/archive")
	flag.Parse()

	opt := logarchive.Options{LogDir: *logDir}
	if *outDir != "" {
		opt.ArchiveDir = *outDir
	}
	outs, err := logarchive.Run(opt)
	if err != nil {
		log.Fatalf("[logarchive] 归档失败: %v", err)
	}
	log.Printf("[logarchive] 完成，本次产出 %d 个归档", len(outs))
}
