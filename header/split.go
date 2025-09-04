package header

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// ==================== 接口定义 ====================

// 数值类型转换接口
type NumberConverter interface {
	ToBytesBE() []byte                      // 转换为大端序字节数组
	ToBytesLE() []byte                      // 转换为小端序字节数组
	FromBytesBE(data []byte) error          // 从大端序字节数组转换
	FromBytesLE(data []byte) error          // 从小端序字节数组转换
	GetBit(bitIndex int) (uint8, error)     // 获取指定位的值
	GetBits(start, end int) (uint64, error) // 获取位区间的值
	ToBinaryString() string                 // 转换为二进制字符串
	ToFloat32() float32                     // 转换为 float32
	ToFloat64() float64                     // 转换为 float64
}

// ==================== 类型定义 ====================

type Uint16 uint16
type Int16 int16
type Uint32 uint32
type Int32 int32
type Uint64 uint64
type Int64 int64
type Float32 float32
type Float64 float64

// ==================== 公共工具函数 ====================

// 检查位索引是否有效
func checkBitIndex(bitIndex, totalBits int) error {
	if bitIndex < 0 || bitIndex >= totalBits {
		return fmt.Errorf("bit index %d out of range [0, %d]", bitIndex, totalBits-1)
	}
	return nil
}

// 检查位区间是否有效
func checkBitRange(start, end, totalBits int) error {
	if start < 0 || start >= totalBits {
		return fmt.Errorf("start bit index %d out of range [0, %d]", start, totalBits-1)
	}
	if end < 0 || end >= totalBits {
		return fmt.Errorf("end bit index %d out of range [0, %d]", end, totalBits-1)
	}
	if start > end {
		return fmt.Errorf("start bit index %d cannot be greater than end bit index %d", start, end)
	}
	return nil
}

// 将字节数组转换为二进制字符串
func bytesToBinaryString(data []byte) string {
	var sb strings.Builder
	for i, b := range data {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(fmt.Sprintf("%08b", b))
	}
	return sb.String()
}

// ==================== Uint16 实现 ====================

func (n Uint16) ToBytesBE() []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(n))
	return buf
}

func (n Uint16) ToBytesLE() []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(n))
	return buf
}

func (n *Uint16) FromBytesBE(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("insufficient data for Uint16")
	}
	*n = Uint16(binary.BigEndian.Uint16(data))
	return nil
}

func (n *Uint16) FromBytesLE(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("insufficient data for Uint16")
	}
	*n = Uint16(binary.LittleEndian.Uint16(data))
	return nil
}

func (n Uint16) GetBit(bitIndex int) (uint8, error) {
	if err := checkBitIndex(bitIndex, 16); err != nil {
		return 0, err
	}
	return uint8((uint16(n) >> (15 - bitIndex)) & 1), nil
}

func (n Uint16) GetBits(start, end int) (uint64, error) {
	if err := checkBitRange(start, end, 16); err != nil {
		return 0, err
	}
	length := end - start + 1
	mask := (uint16(1) << length) - 1
	return uint64((uint16(n) >> (15 - end)) & mask), nil
}

func (n Uint16) ToBinaryString() string {
	return bytesToBinaryString(n.ToBytesBE())
}

func (n Uint16) ToFloat32() float32 {
	return float32(n)
}

func (n Uint16) ToFloat64() float64 {
	return float64(n)
}

// ==================== Int16 实现 ====================

func (n Int16) ToBytesBE() []byte {
	return Uint16(uint16(n)).ToBytesBE()
}

func (n Int16) ToBytesLE() []byte {
	return Uint16(uint16(n)).ToBytesLE()
}

func (n *Int16) FromBytesBE(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("insufficient data for Int16")
	}
	*n = Int16(int16(binary.BigEndian.Uint16(data)))
	return nil
}

func (n *Int16) FromBytesLE(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("insufficient data for Int16")
	}
	*n = Int16(int16(binary.LittleEndian.Uint16(data)))
	return nil
}

func (n Int16) GetBit(bitIndex int) (uint8, error) {
	return Uint16(uint16(n)).GetBit(bitIndex)
}

func (n Int16) GetBits(start, end int) (uint64, error) {
	return Uint16(uint16(n)).GetBits(start, end)
}

func (n Int16) ToBinaryString() string {
	return Uint16(uint16(n)).ToBinaryString()
}

func (n Int16) ToFloat32() float32 {
	return float32(n)
}

func (n Int16) ToFloat64() float64 {
	return float64(n)
}

// ==================== Uint32 实现 ====================

func (n Uint32) ToBytesBE() []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(n))
	return buf
}

func (n Uint32) ToBytesLE() []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(n))
	return buf
}

func (n *Uint32) FromBytesBE(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("insufficient data for Uint32")
	}
	*n = Uint32(binary.BigEndian.Uint32(data))
	return nil
}

func (n *Uint32) FromBytesLE(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("insufficient data for Uint32")
	}
	*n = Uint32(binary.LittleEndian.Uint32(data))
	return nil
}

func (n Uint32) GetBit(bitIndex int) (uint8, error) {
	if err := checkBitIndex(bitIndex, 32); err != nil {
		return 0, err
	}
	return uint8((uint32(n) >> (31 - bitIndex)) & 1), nil
}

func (n Uint32) GetBits(start, end int) (uint64, error) {
	if err := checkBitRange(start, end, 32); err != nil {
		return 0, err
	}
	length := end - start + 1
	mask := (uint32(1) << length) - 1
	return uint64((uint32(n) >> (31 - end)) & mask), nil
}

func (n Uint32) ToBinaryString() string {
	return bytesToBinaryString(n.ToBytesBE())
}

func (n Uint32) ToFloat32() float32 {
	return float32(n)
}

func (n Uint32) ToFloat64() float64 {
	return float64(n)
}

// ==================== Int32 实现 ====================

func (n Int32) ToBytesBE() []byte {
	return Uint32(uint32(n)).ToBytesBE()
}

func (n Int32) ToBytesLE() []byte {
	return Uint32(uint32(n)).ToBytesLE()
}

func (n *Int32) FromBytesBE(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("insufficient data for Int32")
	}
	*n = Int32(int32(binary.BigEndian.Uint32(data)))
	return nil
}

func (n *Int32) FromBytesLE(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("insufficient data for Int32")
	}
	*n = Int32(int32(binary.LittleEndian.Uint32(data)))
	return nil
}

func (n Int32) GetBit(bitIndex int) (uint8, error) {
	return Uint32(uint32(n)).GetBit(bitIndex)
}

func (n Int32) GetBits(start, end int) (uint64, error) {
	return Uint32(uint32(n)).GetBits(start, end)
}

func (n Int32) ToBinaryString() string {
	return Uint32(uint32(n)).ToBinaryString()
}

func (n Int32) ToFloat32() float32 {
	return float32(n)
}

func (n Int32) ToFloat64() float64 {
	return float64(n)
}

// ==================== Uint64 实现 ====================

func (n Uint64) ToBytesBE() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(n))
	return buf
}

func (n Uint64) ToBytesLE() []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(n))
	return buf
}

func (n *Uint64) FromBytesBE(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("insufficient data for Uint64")
	}
	*n = Uint64(binary.BigEndian.Uint64(data))
	return nil
}

func (n *Uint64) FromBytesLE(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("insufficient data for Uint64")
	}
	*n = Uint64(binary.LittleEndian.Uint64(data))
	return nil
}

func (n Uint64) GetBit(bitIndex int) (uint8, error) {
	if err := checkBitIndex(bitIndex, 64); err != nil {
		return 0, err
	}
	return uint8((uint64(n) >> (63 - bitIndex)) & 1), nil
}

func (n Uint64) GetBits(start, end int) (uint64, error) {
	if err := checkBitRange(start, end, 64); err != nil {
		return 0, err
	}
	length := end - start + 1
	mask := (uint64(1) << length) - 1
	return (uint64(n) >> (63 - end)) & mask, nil
}

func (n Uint64) ToBinaryString() string {
	return bytesToBinaryString(n.ToBytesBE())
}

func (n Uint64) ToFloat32() float32 {
	return float32(n)
}

func (n Uint64) ToFloat64() float64 {
	return float64(n)
}

// ==================== Int64 实现 ====================

func (n Int64) ToBytesBE() []byte {
	return Uint64(uint64(n)).ToBytesBE()
}

func (n Int64) ToBytesLE() []byte {
	return Uint64(uint64(n)).ToBytesLE()
}

func (n *Int64) FromBytesBE(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("insufficient data for Int64")
	}
	*n = Int64(int64(binary.BigEndian.Uint64(data)))
	return nil
}

func (n *Int64) FromBytesLE(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("insufficient data for Int64")
	}
	*n = Int64(int64(binary.LittleEndian.Uint64(data)))
	return nil
}

func (n Int64) GetBit(bitIndex int) (uint8, error) {
	return Uint64(uint64(n)).GetBit(bitIndex)
}

func (n Int64) GetBits(start, end int) (uint64, error) {
	return Uint64(uint64(n)).GetBits(start, end)
}

func (n Int64) ToBinaryString() string {
	return Uint64(uint64(n)).ToBinaryString()
}

func (n Int64) ToFloat32() float32 {
	return float32(n)
}

func (n Int64) ToFloat64() float64 {
	return float64(n)
}

// ==================== Float32 实现 ====================

func (n Float32) ToBytesBE() []byte {
	return Uint32(math.Float32bits(float32(n))).ToBytesBE()
}

func (n Float32) ToBytesLE() []byte {
	return Uint32(math.Float32bits(float32(n))).ToBytesLE()
}

func (n *Float32) FromBytesBE(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("insufficient data for Float32")
	}
	uintVal := binary.BigEndian.Uint32(data)
	*n = Float32(math.Float32frombits(uintVal))
	return nil
}

func (n *Float32) FromBytesLE(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("insufficient data for Float32")
	}
	uintVal := binary.LittleEndian.Uint32(data)
	*n = Float32(math.Float32frombits(uintVal))
	return nil
}

func (n Float32) GetBit(bitIndex int) (uint8, error) {
	return Uint32(math.Float32bits(float32(n))).GetBit(bitIndex)
}

func (n Float32) GetBits(start, end int) (uint64, error) {
	return Uint32(math.Float32bits(float32(n))).GetBits(start, end)
}

func (n Float32) ToBinaryString() string {
	return Uint32(math.Float32bits(float32(n))).ToBinaryString()
}

func (n Float32) ToFloat32() float32 {
	return float32(n)
}

func (n Float32) ToFloat64() float64 {
	return float64(n)
}

// ==================== Float64 实现 ====================

func (n Float64) ToBytesBE() []byte {
	return Uint64(math.Float64bits(float64(n))).ToBytesBE()
}

func (n Float64) ToBytesLE() []byte {
	return Uint64(math.Float64bits(float64(n))).ToBytesLE()
}

func (n *Float64) FromBytesBE(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("insufficient data for Float64")
	}
	uintVal := binary.BigEndian.Uint64(data)
	*n = Float64(math.Float64frombits(uintVal))
	return nil
}

func (n *Float64) FromBytesLE(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("insufficient data for Float64")
	}
	uintVal := binary.LittleEndian.Uint64(data)
	*n = Float64(math.Float64frombits(uintVal))
	return nil
}

func (n Float64) GetBit(bitIndex int) (uint8, error) {
	return Uint64(math.Float64bits(float64(n))).GetBit(bitIndex)
}

func (n Float64) GetBits(start, end int) (uint64, error) {
	return Uint64(math.Float64bits(float64(n))).GetBits(start, end)
}

func (n Float64) ToBinaryString() string {
	return Uint64(math.Float64bits(float64(n))).ToBinaryString()
}

func (n Float64) ToFloat32() float32 {
	return float32(n)
}

func (n Float64) ToFloat64() float64 {
	return float64(n)
}

// ==================== 演示代码 ====================

func printBytes(label string, data []byte) {
	fmt.Printf("%s: [", label)
	for i, b := range data {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Printf("%02X", b)
	}
	fmt.Println("]")
}

func printResult(label string, value interface{}) {
	fmt.Printf("%s: %v\n", label, value)
}

func printBinary(label, binaryStr string) {
	fmt.Printf("%s: %s\n", label, binaryStr)
}

func test() {
	fmt.Println("=== 数值类型转换、位操作与浮点数转换演示 ===")

	// 创建测试值
	var u16 Uint16 = 0x1234
	var i16 Int16 = -12345
	var u32 Uint32 = 0x12345678
	var i32 Int32 = -123456789
	var f32 Float32 = 3.14159
	var f64 Float64 = 3.141592653589793

	// 1. 字节数组转换演示
	fmt.Println("\n1. 字节数组转换:")
	printBytes("Uint16 BE", u16.ToBytesBE())
	printBytes("Uint16 LE", u16.ToBytesLE())

	// 2. 位操作演示
	fmt.Println("\n2. 位操作:")

	// 获取单个位
	bit, err := u16.GetBit(3)
	if err == nil {
		printResult("Uint16 第3位的值", bit)
	}

	// 获取位区间
	bits, err := u16.GetBits(4, 11)
	if err == nil {
		printResult("Uint16 第4-11位的值", bits)
	}

	// 3. 二进制表示演示
	fmt.Println("\n3. 二进制表示:")
	printBinary("Uint16 二进制", u16.ToBinaryString())
	printBinary("Int16 二进制", i16.ToBinaryString())
	printBinary("Uint32 二进制", u32.ToBinaryString())
	printBinary("Int32 二进制", i32.ToBinaryString())
	printBinary("Float32 二进制", f32.ToBinaryString())
	printBinary("Float64 二进制", f64.ToBinaryString())

	// 4. 从字节数组转换回数值演示
	fmt.Println("\n4. 从字节数组转换回数值:")

	// Uint16 转换示例
	u16DataBE := u16.ToBytesBE()
	var newU16 Uint16
	newU16.FromBytesBE(u16DataBE)
	printResult("从BE字节数组转换回的Uint16", newU16)

	u16DataLE := u16.ToBytesLE()
	newU16.FromBytesLE(u16DataLE)
	printResult("从LE字节数组转换回的Uint16", newU16)

	// 5. 浮点数转换演示
	fmt.Println("\n5. 浮点数转换:")

	printResult("Uint16 转 Float32", u16.ToFloat32())
	printResult("Uint16 转 Float64", u16.ToFloat64())
	printResult("Int16 转 Float32", i16.ToFloat32())
	printResult("Int16 转 Float64", i16.ToFloat64())
	printResult("Uint32 转 Float32", u32.ToFloat32())
	printResult("Uint32 转 Float64", u32.ToFloat64())
	printResult("Int32 转 Float32", i32.ToFloat32())
	printResult("Int32 转 Float64", i32.ToFloat64())
	printResult("Float32 转 Float32", f32.ToFloat32())
	printResult("Float32 转 Float64", f32.ToFloat64())
	printResult("Float64 转 Float32", f64.ToFloat32())
	printResult("Float64 转 Float64", f64.ToFloat64())

	// 6. 错误处理演示
	fmt.Println("\n6. 错误处理:")

	// 无效位索引
	_, err = u16.GetBit(20)
	if err != nil {
		fmt.Printf("错误: %v\n", err)
	}

	// 无效位区间
	_, err = u16.GetBits(10, 5)
	if err != nil {
		fmt.Printf("错误: %v\n", err)
	}

	// 数据不足
	var testUint16 Uint16
	err = testUint16.FromBytesBE([]byte{0x01})
	if err != nil {
		fmt.Printf("错误: %v\n", err)
	}

	// 7. 浮点数位操作演示
	fmt.Println("\n7. 浮点数位操作:")

	// 获取浮点数的符号位、指数位和尾数位
	sign, _ := f32.GetBit(0)
	exponent, _ := f32.GetBits(1, 8)
	mantissa, _ := f32.GetBits(9, 31)

	printResult("Float32 符号位", sign)
	printResult("Float32 指数位", exponent)
	printResult("Float32 尾数位", mantissa)

	// 8. 浮点数精度演示
	fmt.Println("\n8. 浮点数精度演示:")

	// 大整数转换为浮点数可能丢失精度
	var largeUint64 Uint64 = 1 << 53 // float64 的精度限制
	printResult("Large Uint64", largeUint64)
	printResult("Large Uint64 转 Float64", largeUint64.ToFloat64())
	printResult("Large Uint64 + 1", largeUint64+1)
	printResult("Large Uint64 + 1 转 Float64", (largeUint64 + 1).ToFloat64())
}
