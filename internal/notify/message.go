package notify

import (
	"fmt"

	"github.com/ryabkov82/vff-remnawave-events/internal/remnawave"
)

func TorrentBlockMessage(event remnawave.Event) string {
	duration := event.Data.Report.ActionReport.BlockDuration
	if duration <= 0 {
		duration = 60
	}

	return fmt.Sprintf("⚠️ VPN временно ограничил соединение\n\nМы обнаружили torrent/P2P-трафик через VPN. Такие подключения запрещены правилами сервиса, потому что они создают риск блокировок и проблем для всех пользователей.\n\nДоступ будет автоматически восстановлен примерно через %d секунд.\n\nПожалуйста, отключите torrent-клиент, раздачи, DHT/peer discovery и повторите подключение позже.\n\nЕсли вам нужно использовать торренты, запускайте их только когда VPN работает в режиме прокси для отдельных приложений, а не в режиме полного туннеля. В этом случае torrent-клиент не должен использовать VPN-подключение.", duration)
}
