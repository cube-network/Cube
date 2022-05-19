// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.4;

import "./WithAdmin.sol";
import "./library/SafeSend.sol";
import "./library/initializable.sol";

/*
    DAOCharityFoundation collects and manages part of the block fees for charitable purposes.
*/
contract DAOCharityFoundation is Initializable, SafeSend, WithAdmin {

    event RewardsDistributed(address indexed to, uint256 amount);

    function initialize(address _admin) external initializer {
        admin = _admin;
    }

    function distributeRewards(address payable _to, uint256 _amount) external onlyAdmin {
        sendValue(_to, _amount);
        emit RewardsDistributed(_to, _amount);
    }

    // @dev receive part of block gas fee, actually need to do nothing.
    receive() external payable {}
}