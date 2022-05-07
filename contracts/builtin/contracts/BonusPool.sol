// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.4;

import "./interfaces/IBonusPool.sol";
import "./library/SafeSend.sol";
import "./library/SafeMath.sol";
import "./library/initializable.sol";

contract BonusPool is Initializable, SafeSend, IBonusPool {
    using SafeMath for uint;
    struct BonusRecord {
        address owner;
        // stake in eth
        uint stake;
        // weighted starting timestamp, will be updated when adding new stakes;
        // wst_i = (wst_{i-1} * stake_{i-1} + t_i * deltaStake_i) / (stake_{i-1} + deltaStake_i )
        uint weightedStartTime;
    }

    uint private constant OneYear = 365;
    uint private constant TwoYear = 730;
    uint private constant ThreeYear = 1095;

    address public owner;
    bool public bonusEnded;

    mapping(address => BonusRecord) public records;

    event BonusRecordUpdated(address indexed owner, uint stake, uint time);
    event BonusSend(address indexed owner, address indexed to, uint bonus);
    event NoBonusOnUnbind(address indexed owner, uint256 unbind, uint durationDays);
    event BonusPoolEmptied(bool empty);

    modifier onlyOwner() {
        require(msg.sender == owner, "E01");
        _;
    }

    function initialize(address _stakingContract) external payable initializer {
        owner = _stakingContract;
        if (address(this).balance == 0) {
            bonusEnded = true;
        }
    }

    // binding stake for bonus
    function bindingStake(address _addr, uint256 _deltaEth) external override onlyOwner {
        if (bonusEnded) {
            // return without any error or revert
            return;
        }
        BonusRecord memory rec = records[_addr];
        if (rec.stake == 0) {
            // new record
            records[_addr] = BonusRecord(_addr, _deltaEth, block.timestamp);
            emit BonusRecordUpdated(_addr, _deltaEth, block.timestamp);
        } else {
            uint wst = rec.weightedStartTime.mul(rec.stake).add(block.timestamp.mul(_deltaEth));
            rec.stake = rec.stake.add(_deltaEth);
            rec.weightedStartTime = wst / rec.stake;
            records[_addr] = rec;
            emit BonusRecordUpdated(_addr, rec.stake, rec.weightedStartTime);
        }
    }
    // unbind stake and get bonus
    function unbindStakeAndGetBonus(address _addr, address payable _recipient, uint256 _deltaEth) external override onlyOwner {
        if (bonusEnded) {
            // return without any error or revert
            return;
        }
        BonusRecord memory rec = records[_addr];
        require(_deltaEth > 0 && rec.stake >= _deltaEth, "E24");
        uint durationDays = block.timestamp.sub(rec.weightedStartTime) / 1 days;
        // APR：
        //   1) duration < 3 months, apr = 0%;
        //   2) 3 m <= duration < 12 m, apr = 3%;
        //   3) 12 m <= duration < 24 m, apr = 6%;
        //   4) 24 m <= duration , apr = 9%; AND duration = min( 36 m, duration);

        // apr base on 100
        uint apr = 0;
        if (durationDays >= TwoYear) {
            if (durationDays > ThreeYear) {
                durationDays = ThreeYear;
            }
            apr = 9;
        } else if (durationDays >= OneYear) {
            apr = 6;
        } else if (durationDays >= 90) {
            apr = 3;
        }

        if (apr > 0) {
            uint stakeWei = _deltaEth.mul(1 ether);
            uint bonusWei = stakeWei.mul(durationDays).mul(apr) / OneYear / 100;
            uint pool = address(this).balance;
            if (bonusWei >= pool) {
                bonusWei = pool;
                bonusEnded = true;
                emit BonusPoolEmptied(true);
            }

            sendValue(_recipient, bonusWei);
            emit BonusSend(_addr, _recipient, bonusWei);
        } else {
            emit NoBonusOnUnbind(_addr, _deltaEth, durationDays);
        }

        rec.stake -= _deltaEth;
        records[_addr] = rec;
    }
}