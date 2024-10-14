package main

import (
	"context"
	"fmt"
	"time"

	g "github.com/AllenDang/giu"
	"gopkg.in/vansante/go-ffprobe.v2"
)

const dragDropLabel = "\n\n\n\n\n\n\n                                              将视频拖放到这里"
const shadersTooltip = "如果您不确定如何选择，请查看项目的GitHub页面"
const encoderTooltip = "选择用于输出文件的编码格式。通常GPU编码速度更快，而CPU编码主要适用于GPU较慢的情况\n" +
	"基于GPU的AV1仅适用于RTX 4000系及以上和RX 6500XT及以上显卡\n" +
	"HDR视频仅AV1编码支持"
const crfTooltip = "恒定比特率因子（CRF）参数。\n不要设置太高 - 不然文件会非常大。" +
	"\n\n正确的取值范围：0 - 51\n如果不清楚如何设置，建议保持为20"
const outputFormatTooltip = "如果输入文件包含字幕流，由于其他格式的限制，输出格式必须选择MKV"
const compatibilityModeTooltip = "仅在进行兼容性故障排除时使用，该模式会禁用大多数功能"
const debugModeTooltip = "显示更详细的日志，便于故障排除和调试"

var mainPos float32 = 580
var tablePos float32 = 1200
var bottomBarPos float32 = 1310
var bottomProgressPos float32 = 120
var bottomSpeedPos float32 = 110

func loop(window *g.MasterWindow) {
	resolutionsNames := make([]string, len(resolutions))
	for index, res := range resolutions {
		resolutionsNames[index] = res.Format()
	}

	shadersNames := make([]string, len(shaders))
	for index, shader := range shaders {
		shadersNames[index] = shader.Name
	}

	availableEncodersNames := make([]string, len(availableEncoders))
	for index, encoder := range availableEncoders {
		availableEncodersNames[index] = encoder.Name
	}

	g.SingleWindow().Layout(
		g.SplitLayout(g.DirectionHorizontal, &mainPos,
			g.SplitLayout(g.DirectionVertical, &tablePos,
				g.Layout{
					g.Table().Flags(g.TableFlagsResizable).Rows(buildTableRows()...).Columns(buildTableColumns()...),
				},
				g.Layout{
					g.Label("设置"),
					g.Label(""),

					g.Label("目标分辨率"),
					g.Combo("##", resolutionsNames[settings.Resolution], resolutionsNames, &settings.Resolution).Size(400),
					g.Label(""),

					g.Label("着色器"),
					g.Tooltip(shadersTooltip),
					g.Combo("##", shaders[settings.Shaders].Name, shadersNames, &settings.Shaders).Size(400),
					g.Tooltip(shadersTooltip),
					g.Label(""),

					g.Label("编码器"),
					g.Tooltip(encoderTooltip),
					g.Combo("##", availableEncoders[settings.Encoder].Name, availableEncodersNames, &settings.Encoder).Size(400),
					g.Tooltip(encoderTooltip),
					g.Label(""),

					g.Label("恒定比特率因子（CRF）"),
					g.Tooltip(crfTooltip),
					g.InputInt(&settings.Crf).Size(400).OnChange(func() { handleMinMax(&settings.Crf, 0, 0, 51, 51) }),
					g.Tooltip(crfTooltip),
					g.Label(""),

					g.Label("输出格式"),
					g.Tooltip(outputFormatTooltip),
					g.Combo("##", outputFormats[settings.OutputFormat], outputFormats, &settings.OutputFormat).Size(400),
					g.Tooltip(outputFormatTooltip),
					g.Label(""),

					g.Checkbox("兼容模式", &settings.CompatibilityMode),
					g.Tooltip(compatibilityModeTooltip),

					g.Checkbox("调试模式", &settings.DebugMode),
					g.Tooltip(debugModeTooltip),

					g.Label(""),

					g.Button(gui.ButtonLabel).OnClick(handleButton).Size(360, 30),
				},
			),
			g.Layout{
				g.Label("日志"),
				g.InputTextMultiline(&gui.Logs).Flags(g.InputTextFlagsReadOnly).Size(1600, 270),
				g.SplitLayout(g.DirectionVertical, &bottomBarPos,
					g.SplitLayout(g.DirectionVertical, &bottomProgressPos,
						g.Label("进度: " + gui.TotalProgress),
						g.ProgressBar(gui.Progress).Overlay(gui.ProgressLabel).Size(1170, 20),
					),
					g.SplitLayout(g.DirectionVertical, &bottomSpeedPos,
						g.Label(gui.CurrentSpeed),
						g.Label(gui.Eta),
					),
				),
			},
		),
	)

	settings.PositionX, settings.PositionY = window.GetPos()
}

func handleDrop(files []string) {
	if processing {
		return
	}

	ffprobe.SetFFProbeBinPath(".\\ffmpeg\\ffprobe.exe")
	ctx, closeCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCtx()

	for _, path := range files {
		handleFfprobe(path, ctx)
	}
}

func handleButton() {
	if processing {
		cancelProcessing()
	} else {
		go startProcessing()
	}
}

func resetUI() {
	gui.CurrentSpeed = "速度:"
	gui.Eta = "预计完成时间:"
	gui.TotalProgress = fmt.Sprintf("%d / %d", calcFinished(), len(animeList))
	g.Update()
}

func buildTableRows() []*g.TableRowWidget {
	rows := make([]*g.TableRowWidget, len(animeList))

	for i, anime := range animeList {
		rows[i] = g.TableRow(
			g.Label(fmt.Sprintf("%d", i+1)),
			g.Label(anime.Name),
			g.Label(formatMillis(anime.Length)),
			g.Label(formatMegabytes(anime.Size)),
			g.Label(fmt.Sprintf("%dx%d", anime.Width, anime.Height)),
			g.Label(string(anime.Status)),
			g.Custom(func() { // Workaround for weird UI bug
				g.Button("移除").Disabled(processing).OnClick(func() { removeAnime(i) }).Build()
			}),
		)
	}

	return rows
}

func buildTableColumns() []*g.TableColumnWidget {
	columns := []*g.TableColumnWidget{
		g.TableColumn("编号").Flags(g.TableColumnFlagsWidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("标题").Flags(g.TableColumnFlagsWidthFixed).InnerWidthOrWeight(550),
		g.TableColumn("时长").Flags(g.TableColumnFlagsWidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("大小").Flags(g.TableColumnFlagsWidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("分辨率").Flags(g.TableColumnFlagsWidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("状态").Flags(g.TableColumnFlagsWidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("操作").Flags(g.TableColumnFlagsWidthFixed).InnerWidthOrWeight(100),
	}

	return columns
}
