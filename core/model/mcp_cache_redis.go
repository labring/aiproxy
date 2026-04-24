package model

import (
	"encoding"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/redis/go-redis/v9"
)

var (
	_ encoding.BinaryMarshaler = GroupMCPStatus(0)
	_ redis.Scanner            = (*GroupMCPStatus)(nil)
	_ encoding.BinaryMarshaler = GroupMCPType("")
	_ redis.Scanner            = (*GroupMCPType)(nil)
	_ encoding.BinaryMarshaler = PublicMCPStatus(0)
	_ redis.Scanner            = (*PublicMCPStatus)(nil)
	_ encoding.BinaryMarshaler = PublicMCPType("")
	_ redis.Scanner            = (*PublicMCPType)(nil)
	_ encoding.BinaryMarshaler = (*GroupMCPProxyConfig)(nil)
	_ redis.Scanner            = (*GroupMCPProxyConfig)(nil)
	_ encoding.BinaryMarshaler = MCPPrice{}
	_ redis.Scanner            = (*MCPPrice)(nil)
	_ encoding.BinaryMarshaler = (*PublicMCPProxyConfig)(nil)
	_ redis.Scanner            = (*PublicMCPProxyConfig)(nil)
	_ encoding.BinaryMarshaler = (*MCPOpenAPIConfig)(nil)
	_ redis.Scanner            = (*MCPOpenAPIConfig)(nil)
	_ encoding.BinaryMarshaler = (*MCPEmbeddingConfig)(nil)
	_ redis.Scanner            = (*MCPEmbeddingConfig)(nil)
)

func (s *GroupMCPStatus) ScanRedis(value string) error {
	v, err := strconv.Atoi(value)
	if err != nil {
		return err
	}

	*s = GroupMCPStatus(v)

	return nil
}

func (s GroupMCPStatus) MarshalBinary() ([]byte, error) {
	return conv.StringToBytes(strconv.Itoa(int(s))), nil
}

func (t *GroupMCPType) ScanRedis(value string) error {
	*t = GroupMCPType(value)
	return nil
}

func (t GroupMCPType) MarshalBinary() ([]byte, error) {
	return conv.StringToBytes(string(t)), nil
}

func (s *PublicMCPStatus) ScanRedis(value string) error {
	v, err := strconv.Atoi(value)
	if err != nil {
		return err
	}

	*s = PublicMCPStatus(v)

	return nil
}

func (s PublicMCPStatus) MarshalBinary() ([]byte, error) {
	return conv.StringToBytes(strconv.Itoa(int(s))), nil
}

func (t *PublicMCPType) ScanRedis(value string) error {
	*t = PublicMCPType(value)
	return nil
}

func (t PublicMCPType) MarshalBinary() ([]byte, error) {
	return conv.StringToBytes(string(t)), nil
}

func (c *GroupMCPProxyConfig) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, c)
}

func (c *GroupMCPProxyConfig) MarshalBinary() ([]byte, error) {
	if c == nil {
		return conv.StringToBytes("null"), nil
	}

	return sonic.Marshal(c)
}

func (p *MCPPrice) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, p)
}

func (p MCPPrice) MarshalBinary() ([]byte, error) {
	return sonic.Marshal(p)
}

func (c *PublicMCPProxyConfig) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, c)
}

func (c *PublicMCPProxyConfig) MarshalBinary() ([]byte, error) {
	if c == nil {
		return conv.StringToBytes("null"), nil
	}

	return sonic.Marshal(c)
}

func (c *MCPOpenAPIConfig) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, c)
}

func (c *MCPOpenAPIConfig) MarshalBinary() ([]byte, error) {
	if c == nil {
		return conv.StringToBytes("null"), nil
	}

	return sonic.Marshal(c)
}

func (c *MCPEmbeddingConfig) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, c)
}

func (c *MCPEmbeddingConfig) MarshalBinary() ([]byte, error) {
	if c == nil {
		return conv.StringToBytes("null"), nil
	}

	return sonic.Marshal(c)
}
