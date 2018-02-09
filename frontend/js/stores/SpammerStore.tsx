///<reference path="../../../node_modules/mobx/lib/api/computed.d.ts"/>
import {action, computed, observable, ObservableMap, runInAction} from "mobx";
import dateformat from 'dateformat';

let MsgType = {
    START: 1,
    STOP: 2,
    METRIC: 3,
    STATE: 4,
};

class StateMsg {
    running: boolean;
}

class WsMsg {
    msg_type: number;
    data: any;
    ts: Date;
}

let MetricType = {
    INC_MILESTONE_BRANCH: 0,
    INC_MILESTONE_TRUNK: 1,
    INC_BAD_TRUNK: 2,
    INC_BAD_BRANCH: 3,
    INC_BAD_TRUNK_AND_BRANCH: 4,
    INC_FAILED_TX: 5,
    INC_SUCCESSFUL_TX: 6,
    SUMMARY: 7,
}

export class TXData {
    hash: string;
    count: number;
    created_on: Date;
}

export class MetricSummary {
    txs_succeeded: number;
    txs_failed: number;
    bad_branch: number;
    bad_trunk: number;
    bad_trunk_and_branch: number;
    milestone_trunk: number;
    milestone_branch: number;
    tps: number;
    error_rate: number;
}

export class Metric {
    id: string;
    kind: number;
    data: any;
    ts: Date;
}

export class SpammerStore {
    @observable running: boolean = false;
    @observable connected: boolean = false;
    @observable metrics: ObservableMap<Metric> = observable.map();
    @observable txs: ObservableMap<Metric> = observable.map();
    @observable last_metric: MetricSummary = new MetricSummary();
    ws: WebSocket = null;
    nextMetricID: number = 0;

    async connect() {
        // pull metrics from server
        this.ws = new WebSocket(`ws://${location.host}/api/spammer`);

        this.ws.onmessage = (e: MessageEvent) => {
            let obj: WsMsg = JSON.parse(e.data);

            switch (obj.msg_type) {
                case MsgType.METRIC:
                    let metric: Metric = obj.data;
                    metric.ts = obj.ts;

                    switch (metric.kind) {

                        case MetricType.SUMMARY:
                            runInAction('add metric', () => {
                                this.nextMetricID++;
                                this.metrics.set(this.nextMetricID.toString(), metric);
                                this.last_metric = metric.data;
                            });
                            break;

                        case MetricType.INC_SUCCESSFUL_TX:
                            runInAction('add tx', () => {
                                let tx: TXData = metric.data;
                                this.txs.set(tx.hash, metric);
                            });
                            break;
                    }

                    break;

                case MsgType.STATE:
                    let stateMsg: StateMsg = obj.data;
                    runInAction('update state', () => {
                        this.running = stateMsg.running;
                    });
                    break;
                default:
                    console.log(obj);
            }
        };

        this.ws.onopen = (e) => {
            runInAction('websocket open', () => {
                this.connected = true;
            });
        };

        this.ws.onclose = (e) => {
            runInAction('websocket closed', () => {
                this.connected = false;
            });
        };
    }

    async start() {
        if (!this.connected) return;
        let msg = new WsMsg();
        msg.msg_type = MsgType.START;
        this.ws.send(JSON.stringify(msg));
    }

    async stop() {
        if (!this.connected) return;
        let msg = new WsMsg();
        msg.msg_type = MsgType.STOP;
        this.ws.send(JSON.stringify(msg));
    }

    @computed
    get tps(): Array<any> {
        let a = [];
        this.metrics.forEach(metric => {
            a.push({
                name: dateformat(metric.ts, "HH:MM:ss"),
                value: metric.data.tps,
            });
        });
        a.sort((a,b) => a.ts < b.ts ? 1:-1);
        return a;
    }

    @computed
    get errorRate(): Array<any> {
        let a = [];
        this.metrics.forEach(metric => {
            let d: MetricSummary = metric.data;
            a.push({
                name: dateformat(metric.ts, "HH:MM:ss"),
                ts: metric.ts,
                value: d.error_rate,
            });
        });
        a.sort((a,b) => a.ts < b.ts ? 1:-1);
        return a;
    }

    @computed
    get transactions(): Array<any> {
        let a = [];
        this.txs.forEach(metricTxs => {
            let data: TXData = metricTxs.data;
            data.created_on = metricTxs.ts;
            a.push(data);
        });
        a.sort((a, b) => a.created_on > b.created_on ? -1 : 1);
        return a;
    }

}

export let SpammerStoreInstance = new SpammerStore();