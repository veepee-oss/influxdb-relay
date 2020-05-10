package relay

import (
	"fmt"
	"net/http"

	"github.com/toni-moreno/influxdb-srelay/pkg/relayctx"
	"github.com/toni-moreno/influxdb-srelay/pkg/utils"
)

func (h *HTTP) handlePing(w http.ResponseWriter, r *http.Request) {
	clusterid := relayctx.GetCtxParam(r, "clusterid")
	if len(clusterid) == 0 {
		h.log.Info().Msgf("Handle Health for the hole process....")
		//health for the hole process
		relayctx.SetBackendTime(r)
		utils.AddInfluxPingHeaders(w, "Influx-Smart-Relay")
		relayctx.VoidResponse(w, r, 204)
		return
	}
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Ping for cluster %s", clusterid)
		relayctx.SetBackendTime(r)
		c.HandlePing(w, r)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Ping for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleStatus(w http.ResponseWriter, r *http.Request) {
	clusterid := relayctx.GetCtxParam(r, "clusterid")
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Status for cluster %s", clusterid)
		relayctx.SetBackendTime(r)
		c.HandleStatus(w, r)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Status for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleHealth(w http.ResponseWriter, r *http.Request) {
	clusterid := relayctx.GetCtxParam(r, "clusterid")
	if len(clusterid) == 0 {
		h.log.Info().Msgf("Handle Health for the hole process....")
		//health for the hole process
		relayctx.SetBackendTime(r)
		relayctx.JsonResponse(w, r, 200, "OK")
		return
	}
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Health for cluster %s", clusterid)
		relayctx.SetBackendTime(r)
		c.HandleHealth(w, r)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Health for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleFlush(w http.ResponseWriter, r *http.Request) {

	clusterid := relayctx.GetCtxParam(r, "clusterid")

	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle flush for cluster %s", clusterid)
		relayctx.SetBackendTime(r)
		c.HandleFlush(w, r)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Flush for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleAdmin(w http.ResponseWriter, r *http.Request) {
	clusterid := relayctx.GetCtxParam(r, "clusterid")

	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Admin for cluster %s", clusterid)
		relayctx.SetBackendTime(r)
		c.HandleAdmin(w, r)
		return
	}
	relayctx.JsonResponse(w, r, 400, fmt.Sprintf("cluster %s not exist in  config", clusterid))
	h.log.Error().Msgf("Handle Admin for cluster Error cluster %s not exist", clusterid)
}
