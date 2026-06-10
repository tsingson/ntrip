package ntripio

import (
	"testing"
)

func TestSTRCodec(t *testing.T) {
	// 1. 构造原始结构体实例
	original := &STRRecord{
		Mountpoint:     "RTCM32_VRS",
		Identifier:     "Shanghai",
		Format:         "RTCM 3.2",
		FormatDetails:  "1004(1),1019(1)",
		Carrier:        3,
		NavSystem:      "GPS+GLONASS+BDS",
		Network:        "CORS_NET",
		Country:        "CHN",
		Latitude:       31.23,
		Longitude:      121.47,
		NMEA:           1, // 需要上传 GGA
		Solution:       1, // 网络RTK
		Generator:      "RTCM_Gen_v2",
		ComprEstim:     "none",
		Authentication: "B",
		Fee:            "Y",
		Bitrate:        9600,
		Misc:           "none",
	}

	// 2. 测试序列化 (Marshal)
	rawLine := MarshalSTR(original)
	expectedLine := "STR;RTCM32_VRS;Shanghai;RTCM 3.2;1004(1),1019(1);3;GPS+GLONASS+BDS;CORS_NET;CHN;31.23;121.47;1;1;RTCM_Gen_v2;none;B;Y;9600;none"

	if rawLine != expectedLine {
		t.Fatalf("Marshal failed.\nGot: %s\nExp: %s", rawLine, expectedLine)
	}
	t.Logf("Serialized Output: %s", rawLine)

	// 3. 测试反序列化 (Unmarshal)
	parsed, err := UnmarshalSTR(rawLine)
	if err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	// 4. 字段等值验证
	if parsed.Mountpoint != original.Mountpoint || parsed.Latitude != original.Latitude || parsed.Bitrate != original.Bitrate {
		t.Fatalf("Fields mismatch after Unmarshal. Parsed: %+v", parsed)
	}
	t.Log("Unmarshal verification successful! Fields match perfectly.")
}
