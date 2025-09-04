package header

import "bufio"

type CoarseGraineder func(reader bufio.Reader) ([]byte, error) //粗粒度解析器

type CollectionController interface {
	Linker
	AddProtocolParser(p *ProtocolParser)  //添加规约解析器
	AddLinker(l *Linker)                  //添加连接控制器
	AddCoarseGraineder(f CoarseGraineder) //添加粗粒度解析器
}
