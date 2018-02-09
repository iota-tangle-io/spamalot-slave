import * as React from 'react';
import {inject, observer} from "mobx-react";
import {withRouter} from "react-router";
import {
    AreaChart, Area, ReferenceLine,
    LineChart, ComposedChart, Brush, XAxis, Line, YAxis,
    CartesianGrid, Tooltip, Legend, ResponsiveContainer
} from 'recharts';
import {SpammerStore} from "../stores/SpammerStore";


interface Props {
    spammerStore?: SpammerStore;
}

@withRouter
@inject("spammerStore")
@observer
export class TPSChart extends React.Component<Props, {}> {
    render() {
        let tpsData = this.props.spammerStore.tps;
        return (
            <div>
                <ResponsiveContainer width="100%" height={200}>
                    <ComposedChart data={tpsData} syncId="tps">
                        <XAxis dataKey="name" label={""} />
                        <YAxis/>
                        <CartesianGrid strokeDasharray="2 2"/>
                        <Tooltip/>
                        <Legend/>
                        <Line type="linear" isAnimationActive={false} name="TPS" dataKey="value"
                              fill="#27da9f" stroke="#27da9f" dot={false} activeDot={{r: 8}}/>
                    </ComposedChart>
                </ResponsiveContainer>
            </div>
        );
    }
}