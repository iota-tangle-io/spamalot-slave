import * as React from 'react';
import {Link} from 'react-router-dom';

interface Props {

}

export class Nav extends React.Component<Props, {}> {
    render() {
        return (
            <div className={'nav'}>
                <div className={'site_title'}>Distributed Load Generator</div>
            </div>
        );
    }
}