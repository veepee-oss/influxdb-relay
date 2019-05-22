package relay

import (
	"fmt"
	"net/http"
	"time"

	"github.com/toni-moreno/influxdb-srelay/relayctx"
	"github.com/toni-moreno/influxdb-srelay/utils"
)

func (h *HTTP) handlePing(w http.ResponseWriter, r *http.Request, start time.Time) {
	clusterid := relayctx.GetCtxParam(r, "clusterid")
	if len(clusterid) == 0 {
		h.log.Info().Msgf("Handle Health for the hole process....")
		//health for the hole process
		utils.AddInfluxPingHeaders(w, "Influx-Smart-Relay")
		relayctx.VoidResponse(w, r, 200)
		return
	}
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Ping for cluster %s", clusterid)
		c.HandlePing(w, r, start)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Ping for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleStatus(w http.ResponseWriter, r *http.Request, start time.Time) {
	clusterid := relayctx.GetCtxParam(r, "clusterid")
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Status for cluster %s", clusterid)
		c.HandleStatus(w, r, start)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Status for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleHealth(w http.ResponseWriter, r *http.Request, start time.Time) {
	clusterid := relayctx.GetCtxParam(r, "clusterid")
	if len(clusterid) == 0 {
		h.log.Info().Msgf("Handle Health for the hole process....")
		//health for the hole process
		relayctx.JsonResponse(w, r, 200, "OK")
		return
	}
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Health for cluster %s", clusterid)
		c.HandleHealth(w, r, start)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Health for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleFlush(w http.ResponseWriter, r *http.Request, start time.Time) {

	clusterid := relayctx.GetCtxParam(r, "clusterid")

	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle flush for cluster %s", clusterid)
		c.HandleFlush(w, r, start)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Flush for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleAdmin(w http.ResponseWriter, r *http.Request, start time.Time) {
	clusterid := relayctx.GetCtxParam(r, "clusterid")

	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Admin for cluster %s", clusterid)
		c.HandleAdmin(w, r, start)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Admin for cluster Error cluster %s not exist", clusterid)
}
