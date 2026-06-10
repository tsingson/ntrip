package ntripio

import (
	"fmt"
	"strconv"
	"strings"
)

// SourceTable 代表完整的源列表
type SourceTable struct {
	CAS []CASRecord // Caster 广播器信息
	NET []NETRecord // 网络/运营商信息
	STR []STRRecord // 数据流/挂载点信息（最常用）
}

// CASRecord - Caster 行 (Format: CAS;host;port;identifier;operator;nmea;country;latitude;longitude;fallbackHost;fallbackPort)
type CASRecord struct {
	Host         string  `json:"host"`
	Port         int     `json:"port"`
	Identifier   string  `json:"identifier"`
	Operator     string  `json:"operator"`
	NMEA         int     `json:"nmea"` // 0: 不接收, 1: 接收
	Country      string  `json:"country"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	FallbackHost string  `json:"fallback_host"`
	FallbackPort int     `json:"fallback_port"`
}

// NETRecord - Network 行 (Format: NET;identifier;operator;authentication;fee;web-net;web-reg;web-dl;spatial;networks)
type NETRecord struct {
	Identifier     string `json:"identifier"`
	Operator       string `json:"operator"`
	Authentication string `json:"authentication"` // N: None, B: Basic, D: Digest
	Fee            string `json:"fee"`            // N: Free, Y: Chargeable
	WebNet         string `json:"web_net"`
	WebReg         string `json:"web_reg"`
	WebDl          string `json:"web_dl"`
	Spatial        string `json:"spatial"`
	Networks       string `json:"networks"`
}

// STRRecord - Stream 行，代表一个具体的挂载点 Mountpoint
// Format: STR;mountpoint;identifier;format;format-details;carrier;nav-system;network;country;latitude;longitude;nmea;solution;generator;compr-estim;authentication;fee;bitrate;misc
type STRRecord struct {
	Mountpoint     string  `json:"mountpoint"`     // 挂载点唯一名称
	Identifier     string  `json:"identifier"`     // 城市或地名简称
	Format         string  `json:"format"`         // 数据格式，如 RTCM 3.2, RTCM 3.3, CMR
	FormatDetails  string  `json:"format_details"` // 详细频点信息，如 1004(1),1019(1)
	Carrier        int     `json:"carrier"`        // 载波波段：0:无, 1:单频(L1), 2:双频(L1+L2), 3:多频
	NavSystem      string  `json:"nav_system"`     // 卫星系统：GPS+GLONASS+GALILEO+BDS
	Network        string  `json:"network"`        // 所属网络机构名称
	Country        string  `json:"country"`        // 国家/地区三位代码，如 CHN, USA
	Latitude       float64 `json:"latitude"`       // 纬度
	Longitude      float64 `json:"longitude"`      // 经度
	NMEA           int     `json:"nmea"`           // 是否需要 Rover 上传 GGA：0:不需要, 1:需要(如VRS系统)
	Solution       int     `json:"solution"`       // 解算类型：0:单站, 1:网络RTK
	Generator      string  `json:"generator"`      // 硬件/软件生成器名称
	ComprEstim     string  `json:"compr_estim"`    // 压缩算法
	Authentication string  `json:"authentication"` // 鉴权方式：N: 无, B: Basic, D: Digest
	Fee            string  `json:"fee"`            // 计费：N: 免费, Y: 收费
	Bitrate        int     `json:"bitrate"`        // 比特率 (bps)
	Misc           string  `json:"misc"`           // 杂项备注
}

// MarshalSTR 将结构体序列化为标准 NTRIP STR 文本行
func MarshalSTR(r *STRRecord) string {
	return fmt.Sprintf("STR;%s;%s;%s;%s;%d;%s;%s;%s;%.2f;%.2f;%d;%d;%s;%s;%s;%s;%d;%s",
		r.Mountpoint, r.Identifier, r.Format, r.FormatDetails, r.Carrier,
		r.NavSystem, r.Network, r.Country, r.Latitude, r.Longitude,
		r.NMEA, r.Solution, r.Generator, r.ComprEstim, r.Authentication,
		r.Fee, r.Bitrate, r.Misc)
}

// UnmarshalSTR 将标准 NTRIP STR 文本行反序列化为结构体
func UnmarshalSTR(line string) (*STRRecord, error) {
	if !strings.HasPrefix(line, "STR;") {
		return nil, fmt.Errorf("invalid STR prefix")
	}

	parts := strings.Split(line, ";")
	// 根据标准，STR 行包含 1 + 18 (Type + 18个字段) = 19 个部分
	if len(parts) < 19 {
		return nil, fmt.Errorf("insufficient fields, expected 19, got %d", len(parts))
	}

	var err error
	r := &STRRecord{
		Mountpoint:     parts[1],
		Identifier:     parts[2],
		Format:         parts[3],
		FormatDetails:  parts[4],
		NavSystem:      parts[6],
		Network:        parts[7],
		Country:        parts[8],
		Generator:      parts[13],
		ComprEstim:     parts[14],
		Authentication: parts[15],
		Fee:            parts[16],
		Misc:           parts[18],
	}

	if r.Carrier, err = strconv.Atoi(parts[5]); err != nil {
		return nil, fmt.Errorf("parse carrier err: %v", err)
	}
	if r.Latitude, err = strconv.ParseFloat(parts[9], 64); err != nil {
		return nil, fmt.Errorf("parse latitude err: %v", err)
	}
	if r.Longitude, err = strconv.ParseFloat(parts[10], 64); err != nil {
		return nil, fmt.Errorf("parse longitude err: %v", err)
	}
	if r.NMEA, err = strconv.Atoi(parts[11]); err != nil {
		return nil, fmt.Errorf("parse nmea err: %v", err)
	}
	if r.Solution, err = strconv.Atoi(parts[12]); err != nil {
		return nil, fmt.Errorf("parse solution err: %v", err)
	}
	if r.Bitrate, err = strconv.Atoi(parts[17]); err != nil {
		return nil, fmt.Errorf("parse bitrate err: %v", err)
	}

	return r, nil
}
