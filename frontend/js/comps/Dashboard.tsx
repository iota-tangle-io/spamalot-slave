import * as React from 'react';
import {inject, observer} from "mobx-react";
import Grid from "material-ui/Grid";
import {SpammerStore} from "../stores/SpammerStore";
import Button from "material-ui/Button";
import withStyles from "material-ui/styles/withStyles";
import {StyleRulesCallback, Theme} from "material-ui/styles";
import {WithStyles} from "material-ui";
import {TPSChart} from "./TPSChart";
import {ErrorRateChart} from "./ErrorRateChart";
import Paper from "material-ui/Paper";
import Divider from "material-ui/Divider";
import {TXLog} from "./TXLog";

interface Props {
    spammerStore: SpammerStore;
}

const styles: StyleRulesCallback = (theme: Theme) => ({
    container: {
        display: 'flex',
        flexWrap: 'wrap',
    },
    paper: {
        padding: 16,
    },
    root: {
        flexGrow: 1,
        marginTop: theme.spacing.unit * 2,
    },
    button: {
        marginRight: theme.spacing.unit * 2,
    },
    divider: {
        marginTop: theme.spacing.unit * 3,
        marginBottom: theme.spacing.unit * 3,
    },
    lastMetricInfo: {
        marginTop: theme.spacing.unit * 2,
        marginBottom: theme.spacing.unit * 2,
    }
});

@inject("spammerStore")
@observer
class dashboard extends React.Component<Props & WithStyles, {}> {
    componentWillMount() {
        this.props.spammerStore.connect();
    }

    start = () => {
        this.props.spammerStore.start();
    }

    stop = () => {
        this.props.spammerStore.stop();
    }

    render() {
        let {running, connected, last_metric} = this.props.spammerStore;
        let classes = this.props.classes;

        if(!connected) {
            return (
                <Grid>
                    <h1>Waiting for WebSocket connection...</h1>
                </Grid>
            );
        }

        return (
            <Grid>
                <h1>Dashboard</h1>
                <Grid container className={classes.root}>
                    <Grid item xs={12} lg={12}>
                        <Button className={classes.button} onClick={this.start}
                                disabled={running} variant="raised"
                        >
                            <i className="fas fa-play icon_margin_right"></i>
                            Start
                        </Button>

                        <Button className={classes.button} onClick={this.stop}
                                disabled={!running} variant="raised"
                        >
                            <i className="fas fa-stop icon_margin_right"></i>
                            Stop
                        </Button>
                        {
                            last_metric &&
                            <div className={classes.lastMetricInfo}>
                                TPS: {Math.floor(last_metric.tps * 100) / 100},
                                Error Rate: {Math.floor(last_metric.error_rate * 100) / 100},
                                Bad Branch: {last_metric.bad_branch},
                                Bad Trunk: {last_metric.bad_trunk},
                                Bad Branch And Trunk: {last_metric.bad_trunk_and_branch},
                                Milestone Branch: {last_metric.milestone_branch},
                                Milestone Trunk: {last_metric.milestone_trunk},
                                TX succeeded: {last_metric.txs_succeeded},
                                TX failed: {last_metric.txs_failed}.
                            </div>
                        }
                    </Grid>
                </Grid>

                <Grid container className={classes.root}>
                    <Grid item xs={12} lg={6}>
                        <Paper className={classes.paper}>
                            <h3>TPS</h3>
                            <Divider className={classes.divider}/>
                            <TPSChart/>
                        </Paper>
                    </Grid>
                    <Grid item xs={12} lg={6}>
                        <Paper className={classes.paper}>
                            <h3>Error Rate</h3>
                            <Divider className={classes.divider}/>
                            <ErrorRateChart/>
                        </Paper>
                    </Grid>
                </Grid>

                <Grid item xs={12} lg={12} className={classes.root}>
                    <Paper className={classes.paper}>
                        <TXLog/>
                    </Paper>
                </Grid>

            </Grid>
        );
    }
}

export var Dashboard = withStyles(styles)(dashboard);